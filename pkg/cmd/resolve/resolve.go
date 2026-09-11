// Package resolve implements `linkctl resolve`, which traces the full
// redirect chain a link goes through before reaching its real destination.
package resolve

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/report"
	"github.com/space-code/linkctl/internal/resolve"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	maxHops     int
	userAgent   string
	insecure    bool
	timeout     time.Duration
	format      string
	showHeaders bool
}

func NewCmdResolve(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "resolve <link>",
		Short: "Trace a link's full redirect chain to its real destination",
		Long: `Follows HTTP redirects, HTML <meta http-equiv="refresh"> tags, and simple
JS location assignments until the link reaches a custom-scheme deep link
(the app itself) or a page with no further redirect.

This is the piece a plain AASA check can't provide: an AppsFlyer OneLink
(or any short-link service) lives on a template domain several hops away
from the domain that actually serves apple-app-site-association.

Exits with code 0 when the chain resolves cleanly, 1 on a redirect loop,
an unreachable hop, or a chain that never reaches a final destination.`,
		Args: cobra.ExactArgs(1),
		Example: `  linkctl resolve https://example.onelink.me/abc123
  linkctl resolve https://bit.ly/xyz --user-agent android
  linkctl resolve https://example.com/go --max-hops 5 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args[0], opts)
		},
	}

	cmd.Flags().IntVar(&opts.maxHops, "max-hops", resolve.DefaultMaxHops, "Maximum number of redirects to follow")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "ios", "User-Agent preset (ios, android, desktop, bot) or a custom string")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", httpx.DefaultTimeout, "Per-request HTTP timeout")
	cmd.Flags().StringVar(&opts.format, "format", report.FormatText, "Output format: text, json, github, junit")
	cmd.Flags().BoolVar(&opts.showHeaders, "show-headers", false, "Include each hop's Location header value in text output")

	return cmd
}

func run(f *cmdutil.Factory, link string, opts *options) error {
	if !report.IsValidFormat(opts.format) {
		return fmt.Errorf("invalid --format %q: must be one of %s", opts.format, strings.Join(report.ValidFormats, ", "))
	}

	client := httpx.New(httpx.Options{
		FollowRedirects: false,
		Insecure:        opts.insecure,
		Timeout:         opts.timeout,
		UserAgent:       httpx.ResolveUserAgent(opts.userAgent),
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout*time.Duration(opts.maxHops+1))
	defer cancel()

	trace, err := resolve.Follow(ctx, client, link, opts.maxHops)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	r := report.New("resolve", link)
	i := r.AddSection("Redirect Chain")
	for n, hop := range trace.Hops {
		r.AddKV(i, fmt.Sprintf("Hop %d", n+1), hopLine(hop, opts.showHeaders))
	}
	r.AddKV(i, "Final", trace.Final)
	r.AddChecks(i, resolve.Check(trace)...)

	if err := report.Render(f.IOStreams.Out, f.IOStreams.ColorScheme(), opts.format, r); err != nil {
		return err
	}

	if !r.OK() {
		return cmdutil.ErrChecksFailed
	}
	return nil
}

func hopLine(h resolve.Hop, showHeaders bool) string {
	var sb strings.Builder
	sb.WriteString(string(h.Kind))
	if h.StatusCode != 0 {
		sb.WriteString(" ")
		sb.WriteString(strconv.Itoa(h.StatusCode))
	}
	sb.WriteString(" ")
	sb.WriteString(h.URL)
	if showHeaders && h.Location != "" {
		sb.WriteString(" -> ")
		sb.WriteString(h.Location)
	}
	if h.TLSError != "" {
		sb.WriteString(" [TLS error: " + h.TLSError + "]")
	}
	if h.FetchError != "" {
		sb.WriteString(" [error: " + h.FetchError + "]")
	}
	return sb.String()
}
