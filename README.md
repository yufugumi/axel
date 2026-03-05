# Wellington Axe Runners (WAXE)

WAXE is a Go-based accessibility scanner that uses [chromedp](https://github.com/chromedp/chromedp) with [axe-core](https://github.com/dequelabs/axe-core). It runs monthly via GitHub Actions against six Wellington Council sites and publishes HTML reports to GitHub Releases (no CSV output). The CLI runs locally and does not depend on GitHub Actions.

## Current behavior

- Runs monthly via GitHub Actions against six Wellington Council sites.
- Uses 10 concurrent workers by default.
- URLs are chunked into batches of 50 with exponential delays between chunks (base 250ms, max 2s).
- Retries are disabled by default (set --retries to enable).
- Reuses a single browser process per scan and blocks images/video/audio to speed up scanning.
- Outputs HTML reports into GitHub Releases.

## Run locally

The `axed` CLI runs locally and writes HTML reports to your current working directory by default.

While scans run, the CLI prints a single-line progress update with processed/total counts, percent, and current URL. Press Ctrl+C to cancel a scan.

```bash
go test ./...
go build -o axed ./cmd/scanner
./axed scan --site=wellington
```

Scan a site via sitemap URL (no site config required):

```bash
go build -o axed ./cmd/scanner
./axed scan --sitemap-url https://example.com/sitemap.xml
```

Scan a site via positional base URL (auto-discover sitemaps, then crawl if none found):

```bash
go build -o axed ./cmd/scanner
./axed scan https://example.com
```

> [!NOTE]
> Positional base URLs are mutually exclusive with `--site`, `--sitemap-url`, and `--base-url`. Use one approach per scan.

Sitemap discovery order when using a positional URL:

1. `robots.txt` Sitemap entries (respects redirects)
2. `https://example.com/sitemap.xml`
3. `https://example.com/sitemap_index.xml`
4. `https://example.com/sitemap-index.xml`

If discovery yields no URLs, `axed` crawls the site (same host only) to build a URL list. Crawling respects `robots.txt`, uses a breadth-first traversal with depth 5, delays 300ms between requests, skips non-HTML responses, and de-duplicates URLs. `WAXE_MAX_URLS` still applies to the final list.

Optionally override the base URL for relative sitemap entries and output dir:

```bash
go build -o axed ./cmd/scanner
WAXE_OUTPUT_DIR=./reports ./axed scan --sitemap-url https://example.com/sitemap.xml --base-url https://example.com
```

Set a per-URL timeout (Go duration format):

```bash
go build -o axed ./cmd/scanner
./axed scan --site=wellington --timeout 45s
```

Available scan flags:

- `--workers` (default: 10)
- `--retries` (default: 0)
- `--retry-delay` (default: 2s)
- `--chunk-delay` (default: 250ms)
- `--chunk-delay-max` (default: 2s)
- `--base-url` (only with `--sitemap-url`; positional base URL and `--base-url` are mutually exclusive)

Tune concurrency and retry settings:

```bash
go build -o axed ./cmd/scanner
./axed scan --site=wellington --workers 6 --retries 1 --retry-delay 1s
```

Adjust chunk pacing for large sitemaps:

```bash
go build -o axed ./cmd/scanner
./axed scan --site=wellington --chunk-delay 250ms --chunk-delay-max 2s
```

## Environment overrides

- `WAXE_SITEMAP_URL`
- `WAXE_BASE_URL`
- `WAXE_FALLBACK_URLS`
- `WAXE_FALLBACK_URLS_FILE`
- `WAXE_MAX_URLS`
- `WAXE_WORKERS`
- `WAXE_RETRIES`
- `WAXE_RETRY_DELAY`
- `WAXE_CHUNK_DELAY`
- `WAXE_CHUNK_DELAY_MAX`
- `WAXE_OUTPUT_DIR`
- `CHROME_PATH`
- `WAXE_ALLOW_SITEMAP_OVERRIDE` (set to `true` to allow `WAXE_SITEMAP_URL` with `--site`)

> [!NOTE]
> `WAXE_SITEMAP_URL` is only used when `--site` is not provided. Use `--sitemap-url` to override the sitemap, or set `WAXE_ALLOW_SITEMAP_OVERRIDE=true` to opt into environment overrides while scanning a configured site.

> [!NOTE]
> This project is still a work in progress and has rough edges. It can scan other sites with axe-core, but it needs more configuration and polish to be broadly reusable.

## Known issues

- Some sites have slow or flaky pages that can still time out; enable retries if needed.
- The HTML reports are useful but still need refinement for long-term analysis and comparison.
- Runtime can be slow on large sitemaps even with concurrency.
