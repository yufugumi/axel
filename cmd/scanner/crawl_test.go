package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDiscoverSitemapURLsFromRobots(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nSitemap: /sitemap.xml\nSitemap: /sitemap-extra.xml\n"))
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		case "/sitemap-extra.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	urls, err := discoverSitemapURLs(context.Background(), parsed)
	if err != nil {
		t.Fatalf("discover sitemaps: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if urls[0] != server.URL+"/page" {
		t.Fatalf("expected sitemap URL %s/page, got %s", server.URL, urls[0])
	}
}

func TestDiscoverSitemapURLsDefaultsWhenRobotsMissing(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.NotFound(w, r)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	urls, err := discoverSitemapURLs(context.Background(), parsed)
	if err != nil {
		t.Fatalf("discover sitemaps: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if urls[0] != server.URL+"/page" {
		t.Fatalf("expected sitemap URL %s/page, got %s", server.URL, urls[0])
	}
}

func TestCrawlSiteRespectsRobotsAndHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body>
				<a href="/allowed">Allowed</a>
				<a href="/allowed#section">Allowed fragment</a>
				<a href="/private">Private</a>
				<a href="/file.pdf">PDF</a>
				<a href="/missing-type">Missing type</a>
				<a href="https://example.com/out">External</a>
			</body></html>`))
		case "/allowed":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body><a href="/deep">Deep</a></body></html>`))
		case "/deep":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body>Deep</body></html>`))
		case "/private":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body>Private</body></html>`))
		case "/file.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4"))
		case "/missing-type":
			_, _ = w.Write([]byte("<!doctype html><html><body>Missing type</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	urls, err := crawlSite(context.Background(), parsed, crawlOptions{MaxDepth: 3, Delay: 0})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}
	if !containsURL(urls, server.URL+"/") {
		t.Fatalf("expected root URL in crawl results")
	}
	if !containsURL(urls, server.URL+"/allowed") {
		t.Fatalf("expected allowed URL in crawl results")
	}
	if !containsURL(urls, server.URL+"/deep") {
		t.Fatalf("expected deep URL in crawl results")
	}
	if containsURL(urls, server.URL+"/private") {
		t.Fatalf("expected private URL to be excluded by robots")
	}
	if containsURL(urls, server.URL+"/file.pdf") {
		t.Fatalf("expected non-HTML URL to be excluded")
	}
	if !containsURL(urls, server.URL+"/missing-type") {
		t.Fatalf("expected missing content-type HTML to be included")
	}
}

func TestDiscoverSitemapURLsMergesAllCandidates(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	otherSitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/other</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nSitemap: /sitemap.xml\n"))
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		case "/sitemap_index.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(otherSitemap))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	urls, err := discoverSitemapURLs(context.Background(), parsed)
	if err != nil {
		t.Fatalf("discover sitemaps: %v", err)
	}
	if !containsURL(urls, server.URL+"/page") {
		t.Fatalf("expected sitemap URL %s/page", server.URL)
	}
	if !containsURL(urls, server.URL+"/other") {
		t.Fatalf("expected sitemap URL %s/other", server.URL)
	}
}

func TestCrawlSiteSuppresses404RobotsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body>Root</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	logs := captureLogs(t)
	_, err = crawlSite(context.Background(), parsed, crawlOptions{MaxDepth: 1, Delay: 0})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}
	if strings.Contains(logs.String(), "robots fetch failed") {
		t.Fatalf("expected 404 robots warning to be suppressed, got %s", logs.String())
	}
}

func TestCrawlSiteWarnsWhenRobotsFetchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusInternalServerError)
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body>Root</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	logs := captureLogs(t)
	_, err = crawlSite(context.Background(), parsed, crawlOptions{MaxDepth: 1, Delay: 0})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}
	if !strings.Contains(logs.String(), "robots fetch failed") {
		t.Fatalf("expected robots fetch warning, got %s", logs.String())
	}
}

func captureLogs(t *testing.T) *strings.Builder {
	t.Helper()
	var buffer strings.Builder
	logger := log.Default()
	previous := logger.Writer()
	logger.SetOutput(&buffer)
	t.Cleanup(func() {
		logger.SetOutput(previous)
	})
	return &buffer
}

func containsURL(urls []string, target string) bool {
	for _, candidate := range urls {
		if strings.TrimRight(candidate, "/") == strings.TrimRight(target, "/") {
			return true
		}
	}
	return false
}
