# linkctl

A CLI for validating iOS deep links — Universal Links, custom schemes, and
AppsFlyer OneLink — so misconfiguration is caught in CI instead of on a
tester's phone.

## Install

```sh
go install github.com/space-code/linkctl/cmd@latest
```

or build from source:

```sh
make build   # ./bin/linkctl
```

## Commands

| Command | What it checks |
|---|---|
| `linkctl check-app <link>` | Whether an Xcode project is configured to handle a link (entitlements, URL schemes, Info.plist) — no network required. |
| `linkctl scan [project]` | Every deep link pattern an Xcode project can handle. |
| `linkctl validate <link>` | Server-side AASA for one link: TLS, HTTP response, JSON structure, and whether the link's path is covered. |
| `linkctl aasa <domain\|link>` | A deeper AASA inspector: fetches the well-known path, the legacy root path, and Apple's CDN, and reports every appID/path pattern declared. |
| `linkctl resolve <link>` | Traces the full redirect chain (HTTP 3xx, meta-refresh, JS) to a link's real destination. |
| `linkctl onelink <link>` | Validates an AppsFlyer OneLink URL end-to-end: structure, deep-linking params, and — by following the redirect — whether the landing domain's AASA actually covers it. |
| `linkctl open <link>` | Opens a link on a booted iOS simulator via `simctl`, to confirm it actually routes into the app. |
| `linkctl cache-reset` | Resets the iOS Universal Links cache on a simulator. |
| `linkctl ci` | Runs every configured check against a list of links from `linkctl.yml` in one step. |
| `linkctl devices` | Lists booted iOS simulators. |

Every network-facing command supports `--insecure` (skip TLS verification,
for a staging environment with a self-signed cert), `--timeout`, and
`--user-agent ios|android|desktop|bot|<custom>` (many redirect/shortlink
services branch on it).

## CI integration

The check-producing commands (`aasa`, `resolve`, `onelink`, `ci`) share a
`--format` flag:

- `text` — colourised terminal output (default)
- `json` — for scripting
- `github` — `::error::`/`::warning::` workflow annotations, so failures show
  up inline on a GitHub Actions run
- `junit` — XML test report for GitLab CI / Jenkins / Buildkite

Every command exits `0` when checks pass and `1` otherwise, so it composes
directly into a CI job:

```yaml
- name: Validate deep links
  run: linkctl ci --format github
```

### `linkctl.yml`

`ci` checks a whole list of links in one step. See
[`linkctl.example.yml`](linkctl.example.yml) for a fully-commented example.
Minimal form:

```yaml
links:
  - url: https://example.com/profile/42
  - url: https://example.onelink.me/abc1/xyz789
    expect: onelink
  - url: myapp://profile/42
    expect: custom-scheme
```

```sh
linkctl ci --config linkctl.yml --format github
linkctl ci --fail-on warning   # also fail the run on WARN-level findings
```

## Development

```sh
make dev     # format, vet, lint, test, build
make test    # go test -v -race ./...
make lint    # golangci-lint
```

See `Makefile` (`make help`) for the full list of targets.
