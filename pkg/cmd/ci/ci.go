// Package ci implements `linkctl ci`, which runs every check this tool
// knows against a whole list of links in one CI step, driven by a
// linkctl.yml config file instead of one invocation per link.
package ci

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/appcheck"
	"github.com/space-code/linkctl/internal/config"
	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/onelink"
	"github.com/space-code/linkctl/internal/parser"
	"github.com/space-code/linkctl/internal/report"
	"github.com/spf13/cobra"

	"github.com/space-code/linkctl/pkg/cmdutil"
)

type options struct {
	config   string
	failOn   string
	format   string
	insecure bool
}

func NewCmdCI(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Run every configured link check in one step",
		Long: `Reads linkctl.yml and runs the appropriate checks (AASA, OneLink,
app-project) against every configured link, so a CI pipeline needs a single
step instead of one linkctl invocation per link.

Exits with code 0 when every link passes, 1 otherwise.`,
		Args: cobra.NoArgs,
		Example: `  linkctl ci
  linkctl ci --config ./configs/linkctl.yml --format github
  linkctl ci --fail-on warning`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.config, "config", "c", "linkctl.yml", "Path to the linkctl.yml config file")
	cmd.Flags().StringVar(&opts.failOn, "fail-on", "error", "Minimum severity that fails the run: error or warning")
	cmd.Flags().StringVar(&opts.format, "format", report.FormatText, "Output format: text, json, github, junit")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")

	return cmd
}

func run(f *cmdutil.Factory, opts *options) error {
	if !report.IsValidFormat(opts.format) {
		return fmt.Errorf("invalid --format %q: must be one of %s", opts.format, strings.Join(report.ValidFormats, ", "))
	}
	if opts.failOn != "error" && opts.failOn != "warning" {
		return fmt.Errorf("invalid --fail-on %q: must be \"error\" or \"warning\"", opts.failOn)
	}

	cfg, err := config.Load(opts.config)
	if err != nil {
		return err
	}

	identity, err := resolveIdentity(cfg)
	if err != nil {
		return err
	}

	timeout, err := cfg.Timeout(httpx.DefaultTimeout)
	if err != nil {
		return err
	}

	client := httpx.New(httpx.Options{
		FollowRedirects: false,
		Insecure:        opts.insecure,
		Timeout:         timeout,
		UserAgent:       httpx.ResolveUserAgent(cfg.UserAgent),
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Duration(len(cfg.Links)*12+1))
	defer cancel()

	r := report.New("ci", opts.config)
	for _, link := range cfg.Links {
		i := r.AddSection(link.URL)
		r.AddChecks(i, checkLink(ctx, client, cfg, link, identity)...)
	}

	if err := report.Render(f.IOStreams.Out, f.IOStreams.ColorScheme(), opts.format, r); err != nil {
		return err
	}

	summary := r.Summary()
	failed := summary.Failed > 0
	if opts.failOn == "warning" {
		failed = failed || summary.Warnings > 0
	}
	if failed {
		return cmdutil.ErrChecksFailed
	}
	return nil
}

func resolveIdentity(cfg *config.Config) (aasa.AppIdentity, error) {
	identity := aasa.AppIdentity{TeamID: cfg.TeamID, BundleID: cfg.BundleID}
	if cfg.Project == "" {
		return identity, nil
	}
	fromProject, err := aasa.IdentityFromProject(cfg.Project, cfg.Target)
	if err != nil {
		// Non-fatal: the project may simply not resolve a single target
		// without --target; per-link checks still run without an expected
		// app identity rather than aborting the whole run.
		return identity, nil //nolint:nilerr
	}
	if identity.TeamID == "" {
		identity.TeamID = fromProject.TeamID
	}
	if identity.BundleID == "" {
		identity.BundleID = fromProject.BundleID
	}
	return identity, nil
}

// checkLink runs the checks appropriate to one configured link and returns
// them all prefixed with the check category, so results from different
// link types read clearly within one section.
func checkLink(ctx context.Context, client *http.Client, cfg *config.Config, link config.LinkConfig, identity aasa.AppIdentity) []models.ValidationResult {
	expect := link.Expect
	if expect == config.ExpectAuto {
		expect = inferExpect(link.URL)
	}

	var results []models.ValidationResult

	switch expect {
	case config.ExpectOneLink:
		results = append(results, checkOneLinkEntry(ctx, client, link, identity)...)
	case config.ExpectCustomScheme:
		results = append(results, models.ValidationResult{
			Check:   "Custom Scheme",
			Status:  models.StatusInfo,
			Message: "no server-side check applies to a custom-scheme link",
		})
	default: // universal-link
		if cfg.Checks.AASAEnabled() {
			results = append(results, checkUniversalLinkEntry(ctx, client, link, identity, cfg.Checks.AppleCDNEnabled())...)
		}
	}

	if cfg.Project != "" && cfg.Checks.AppProjectEnabled() {
		results = append(results, checkAppProjectEntry(cfg, link)...)
	}

	return results
}

func inferExpect(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return config.ExpectCustomScheme
	}
	return config.ExpectUniversalLink
}

func checkUniversalLinkEntry(ctx context.Context, client *http.Client, link config.LinkConfig, identity aasa.AppIdentity, checkCDN bool) []models.ValidationResult {
	u, err := url.Parse(link.URL)
	if err != nil {
		return []models.ValidationResult{{
			Check:   "URL",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("invalid URL: %v", err),
		}}
	}

	var fetches []aasa.Fetch
	if checkCDN {
		fetches = aasa.FetchAll(ctx, client, u.Host)
	} else {
		fetches = []aasa.Fetch{aasa.FetchOne(ctx, client, u.Host, aasa.SourceWellKnown)}
	}

	var targetURL *url.URL
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		targetURL = u
	}

	return aasa.Check(fetches, aasa.CheckOptions{Want: identity, TargetURL: targetURL})
}

func checkOneLinkEntry(ctx context.Context, client *http.Client, link config.LinkConfig, identity aasa.AppIdentity) []models.ValidationResult {
	result, err := onelink.Analyze(ctx, client, link.URL, onelink.Options{Want: identity})
	if err != nil {
		return []models.ValidationResult{{
			Check:   "OneLink",
			Status:  models.StatusFail,
			Message: err.Error(),
		}}
	}

	checks := result.Checks
	if link.FinalHost != "" {
		checks = append(checks, checkFinalHost(result, link.FinalHost))
	}
	return checks
}

func checkFinalHost(result *onelink.Report, wantHost string) models.ValidationResult {
	if result.Trace == nil || result.Trace.Final == "" {
		return models.ValidationResult{
			Check:   "Final Host",
			Status:  models.StatusFail,
			Message: "redirect chain did not resolve",
		}
	}
	finalURL, err := url.Parse(result.Trace.Final)
	if err != nil {
		return models.ValidationResult{
			Check:   "Final Host",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("could not parse final destination %q", result.Trace.Final),
		}
	}
	if !strings.EqualFold(finalURL.Host, wantHost) {
		return models.ValidationResult{
			Check:   "Final Host",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("expected final host %q, got %q", wantHost, finalURL.Host),
			Detail:  result.Trace.Final,
		}
	}
	return models.ValidationResult{
		Check:   "Final Host",
		Status:  models.StatusPass,
		Message: wantHost,
	}
}

func checkAppProjectEntry(cfg *config.Config, link config.LinkConfig) []models.ValidationResult {
	parsedLink, err := parser.Parse(link.URL)
	if err != nil {
		return []models.ValidationResult{{
			Check:   "App Project",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("invalid link: %v", err),
		}}
	}

	appCheckReport, err := appcheck.CheckApp(cfg.Project, parsedLink, cfg.Target, "")
	if err != nil {
		return []models.ValidationResult{{
			Check:   "App Project",
			Status:  models.StatusFail,
			Message: err.Error(),
		}}
	}
	return appCheckReport.Checks
}
