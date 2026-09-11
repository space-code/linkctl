// Package aasa implements `linkctl aasa`, a detailed inspector for
// apple-app-site-association files: it shows every source Apple's iOS
// devices might read (well-known path, legacy root path, Apple's CDN),
// every appID and path pattern declared, and — when the argument includes a
// path — whether that specific link is actually covered.
package aasa

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/parser"
	"github.com/space-code/linkctl/internal/report"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	project   string
	target    string
	bundleID  string
	teamID    string
	source    string
	format    string
	insecure  bool
	timeout   time.Duration
	userAgent string
}

func NewCmdAASA(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "aasa <domain|link>",
		Short: "Inspect apple-app-site-association in detail",
		Long: `Fetches apple-app-site-association from every source that matters —
the well-known path, the legacy root path, and Apple's own CDN — and reports
every appID and path pattern it declares.

When the argument includes a path (e.g. https://example.com/profile/42
instead of just example.com), it also checks whether that specific path is
covered by applinks.details.

Exits with code 0 when every check passes, 1 otherwise.`,
		Args: cobra.ExactArgs(1),
		Example: `  linkctl aasa example.com
  linkctl aasa https://example.com/profile/42 --bundle-id com.example.app --team-id ABCDE12345
  linkctl aasa example.com --project ./MyApp.xcodeproj --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Path to .xcodeproj to derive --bundle-id/--team-id from")
	cmd.Flags().StringVarP(&opts.target, "target", "t", "", "Xcode target name (when --project has multiple targets)")
	cmd.Flags().StringVar(&opts.bundleID, "bundle-id", "", "Expected bundle identifier")
	cmd.Flags().StringVar(&opts.teamID, "team-id", "", "Expected Apple Developer Team ID")
	cmd.Flags().StringVar(&opts.source, "source", "all", "AASA source to fetch: well-known, root, apple-cdn, or all")
	cmd.Flags().StringVar(&opts.format, "format", report.FormatText, "Output format: text, json, github, junit")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", httpx.DefaultTimeout, "HTTP request timeout")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "ios", "User-Agent preset (ios, android, desktop, bot) or a custom string")

	return cmd
}

func run(f *cmdutil.Factory, rawArg string, opts *options) error {
	if !report.IsValidFormat(opts.format) {
		return fmt.Errorf("invalid --format %q: must be one of %s", opts.format, strings.Join(report.ValidFormats, ", "))
	}

	link, err := parser.Parse(rawArg)
	if err != nil {
		return fmt.Errorf("invalid domain or link: %w", err)
	}

	identity, err := resolveIdentity(opts)
	if err != nil {
		return err
	}

	client := httpx.New(httpx.Options{
		Timeout:   opts.timeout,
		Insecure:  opts.insecure,
		UserAgent: httpx.ResolveUserAgent(opts.userAgent),
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout*4)
	defer cancel()

	fetches, err := fetch(ctx, client, link.Host, opts.source)
	if err != nil {
		return err
	}

	var targetURL *url.URL
	if hasPathToMatch(link) {
		targetURL, _ = url.Parse(link.Raw)
	}

	checks := aasa.Check(fetches, aasa.CheckOptions{Want: identity, TargetURL: targetURL})

	r := report.New("aasa", link.Raw)
	i := r.AddSection("AASA")
	for _, fe := range fetches {
		r.AddKV(i, string(fe.Source), fetchSummary(fe))
	}
	if id := identity.AppID(); id != "" {
		r.AddKV(i, "Expected App ID", id)
	}
	r.AddChecks(i, checks...)

	if err := report.Render(f.IOStreams.Out, f.IOStreams.ColorScheme(), opts.format, r); err != nil {
		return err
	}

	if !r.OK() {
		return cmdutil.ErrChecksFailed
	}
	return nil
}

func resolveIdentity(opts *options) (aasa.AppIdentity, error) {
	identity := aasa.AppIdentity{TeamID: opts.teamID, BundleID: opts.bundleID}

	if opts.project != "" {
		fromProject, err := aasa.IdentityFromProject(opts.project, opts.target)
		if err != nil {
			return aasa.AppIdentity{}, fmt.Errorf("reading --project: %w", err)
		}
		if identity.TeamID == "" {
			identity.TeamID = fromProject.TeamID
		}
		if identity.BundleID == "" {
			identity.BundleID = fromProject.BundleID
		}
	}

	return identity, nil
}

func fetch(ctx context.Context, client *http.Client, domain, source string) ([]aasa.Fetch, error) {
	if source == "all" || source == "" {
		return aasa.FetchAll(ctx, client, domain), nil
	}

	src := aasa.Source(source)
	switch src {
	case aasa.SourceWellKnown, aasa.SourceRoot, aasa.SourceAppleCDN:
		return []aasa.Fetch{aasa.FetchOne(ctx, client, domain, src)}, nil
	default:
		return nil, fmt.Errorf("invalid --source %q: must be well-known, root, apple-cdn, or all", source)
	}
}

// hasPathToMatch reports whether the parsed link carries more than a bare
// domain, i.e. there is an actual path/query/fragment worth matching against
// applinks.details. A bare domain (e.g. "example.com") only gets the
// structural checks.
func hasPathToMatch(link *models.DeepLink) bool {
	return (link.Path != "" && link.Path != "/") || len(link.Query) > 0 || link.Fragment != ""
}

func fetchSummary(f aasa.Fetch) string {
	switch {
	case f.TLSError != "":
		return "TLS error"
	case f.FetchError != "":
		return "unreachable"
	case f.Redirected:
		return fmt.Sprintf("HTTP %d redirect -> %s", f.StatusCode, f.Location)
	case f.StatusCode == 0:
		return "not fetched"
	default:
		return fmt.Sprintf("HTTP %d (%d bytes)", f.StatusCode, f.SizeBytes)
	}
}
