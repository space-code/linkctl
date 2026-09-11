// Package httpx builds the *http.Client used by every network-facing
// package in linkctl (validator/aasa, resolve, onelink). Centralising it
// here means the timeout, redirect policy, TLS policy, and User-Agent are
// configured in exactly one place instead of once per package.
package httpx

import (
	"crypto/tls"
	"net/http"
	"time"
)

// DefaultTimeout is used when Options.Timeout is zero.
const DefaultTimeout = 10 * time.Second

// User-Agent presets. AppsFlyer OneLink (and many redirect services) serve a
// different response depending on the client's User-Agent, so callers need
// to be able to pick one deliberately rather than send Go's default
// "Go-http-client/1.1", which most such services do not recognise as a
// mobile browser at all.
const (
	UserAgentIOS     = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	UserAgentAndroid = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36"
	UserAgentDesktop = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
	UserAgentBot     = "linkctl/1.0 (+https://github.com/space-code/linkctl)"
)

// ResolveUserAgent maps a short preset name ("ios", "android", "desktop",
// "bot") to its full User-Agent string. Any other non-empty value is
// returned unchanged, so callers may also pass a fully custom UA string.
// An empty preset resolves to UserAgentIOS, since Universal Links and
// OneLink are both primarily an iOS Safari concern in this tool.
func ResolveUserAgent(preset string) string {
	switch preset {
	case "", "ios":
		return UserAgentIOS
	case "android":
		return UserAgentAndroid
	case "desktop":
		return UserAgentDesktop
	case "bot":
		return UserAgentBot
	default:
		return preset
	}
}

// Options configures the client returned by New.
type Options struct {
	// Timeout is the total request timeout. Defaults to DefaultTimeout.
	Timeout time.Duration

	// FollowRedirects controls whether the client transparently follows
	// HTTP 3xx responses. AASA checks want this false (a redirect on the
	// well-known endpoint is itself a failure per Apple's spec); redirect
	// tracing wants this false too, so it can inspect each hop manually.
	// It exists mainly so call sites document their choice explicitly.
	FollowRedirects bool

	// Insecure disables TLS certificate verification. Off by default —
	// Universal Links require a valid certificate, so silently accepting
	// an invalid one would hide a real misconfiguration. Only meant for
	// debugging a staging environment with a self-signed certificate.
	Insecure bool

	// UserAgent is sent as the User-Agent header on every request. Empty
	// means "use net/http's default", which is rarely what callers here
	// want — prefer ResolveUserAgent("ios") or similar.
	UserAgent string
}

// userAgentTransport injects a fixed User-Agent header, since http.Client
// has no built-in per-client default for it.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(req)
}

// New builds an *http.Client configured per opts. Redirects are never
// followed automatically by the returned client's Client.Get/Do when
// FollowRedirects is false — callers that need to observe redirects
// (internal/resolve) must set FollowRedirects to true, or drive the
// transport directly.
func New(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.Insecure, //nolint:gosec // explicit opt-in via --insecure
		},
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: &userAgentTransport{base: transport, userAgent: opts.UserAgent},
	}

	if !opts.FollowRedirects {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client
}
