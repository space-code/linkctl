// Package onelink implements `linkctl onelink`, which analyzes an
// AppsFlyer OneLink URL end-to-end: its template/shortlink structure, the
// deep-linking query parameters, and — by following the redirect chain —
// whether the domain it actually lands on serves an AASA that covers it.
package onelink

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/onelink"
	"github.com/space-code/linkctl/internal/report"
	"github.com/space-code/linkctl/internal/resolve"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	project   string
	target    string
	bundleID  string
	teamID    string
	noResolve bool
	maxHops   int
	insecure  bool
	timeout   time.Duration
	userAgent string
	format    string
}

func NewCmdOneLink(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "onelink <link>",
		Short: "Validate an AppsFlyer OneLink URL end-to-end",
		Long: `Analyzes an AppsFlyer OneLink URL: its template/shortlink structure, the
deep-linking query parameters (deep_link_value/af_dp, af_web_dp, pid, c,
af_force_deeplink), whether the OneLink domain itself serves an AASA, and —
by following the redirect chain — whether the domain it actually lands on
serves an AASA that covers the final path.

Exits with code 0 when every check passes, 1 otherwise.`,
		Args: cobra.ExactArgs(1),
		Example: `  linkctl onelink https://example.onelink.me/abc123/xyz789
  linkctl onelink https://example.onelink.me/abc123/xyz789 --project ./MyApp.xcodeproj
  linkctl onelink https://example.onelink.me/abc123/xyz789 --no-resolve --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Path to .xcodeproj to derive --bundle-id/--team-id from")
	cmd.Flags().StringVarP(&opts.target, "target", "t", "", "Xcode target name (when --project has multiple targets)")
	cmd.Flags().StringVar(&opts.bundleID, "bundle-id", "", "Expected bundle identifier")
	cmd.Flags().StringVar(&opts.teamID, "team-id", "", "Expected Apple Developer Team ID")
	cmd.Flags().BoolVar(&opts.noResolve, "no-resolve", false, "Skip following the redirect chain (structure/parameter checks only)")
	cmd.Flags().IntVar(&opts.maxHops, "max-hops", resolve.DefaultMaxHops, "Maximum number of redirects to follow")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", httpx.DefaultTimeout, "Per-request HTTP timeout")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "ios", "User-Agent preset (ios, android, desktop, bot) or a custom string")
	cmd.Flags().StringVar(&opts.format, "format", report.FormatText, "Output format: text, json, github, junit")

	return cmd
}

func run(f *cmdutil.Factory, link string, opts *options) error {
	if !report.IsValidFormat(opts.format) {
		return fmt.Errorf("invalid --format %q: must be one of %s", opts.format, strings.Join(report.ValidFormats, ", "))
	}

	identity, err := resolveIdentity(opts)
	if err != nil {
		return err
	}

	client := httpx.New(httpx.Options{
		FollowRedirects: false,
		Insecure:        opts.insecure,
		Timeout:         opts.timeout,
		UserAgent:       httpx.ResolveUserAgent(opts.userAgent),
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout*time.Duration(opts.maxHops+3))
	defer cancel()

	result, err := onelink.Analyze(ctx, client, link, onelink.Options{
		Want:        identity,
		MaxHops:     opts.maxHops,
		SkipResolve: opts.noResolve,
	})
	if err != nil {
		return fmt.Errorf("onelink: %w", err)
	}

	r := report.New("onelink", link)
	i := r.AddSection("OneLink")
	r.AddKV(i, "Domain", result.Domain)
	if result.TemplateID != "" {
		r.AddKV(i, "Template", result.TemplateID)
	}
	if result.ShortlinkID != "" {
		r.AddKV(i, "Shortlink", result.ShortlinkID)
	}
	if id := identity.AppID(); id != "" {
		r.AddKV(i, "Expected App ID", id)
	}
	if result.Trace != nil {
		r.AddKV(i, "Final Destination", result.Trace.Final)
	}
	r.AddChecks(i, result.Checks...)

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
