package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestCanonicalMenuURL(t *testing.T) {
	base := mustURL(t, "https://example.com/root")
	_, first, ok := canonicalMenuURL("/page?b=2&a=1#section", base)
	if !ok {
		t.Fatal("first URL rejected")
	}
	_, second, ok := canonicalMenuURL("/page?a=1&b=2", base)
	if !ok || first != second {
		t.Fatalf("canonical URLs differ: %q vs %q", first, second)
	}
	if _, _, ok := canonicalMenuURL("https://other.example/page", base); ok {
		t.Fatal("external origin accepted")
	}
	_, route, ok := canonicalMenuURL("/app#/settings", base)
	if !ok || !strings.Contains(route, "#/settings") {
		t.Fatalf("hash route lost: %q", route)
	}
}

func TestIprovestInlineMenus(t *testing.T) {
	root := mustURL(t, "https://www.iprovest.com/")
	cases := []struct {
		handler string
		source  string
		page    string
		id      string
		inner   string
	}{
		{"usrBody('800','/customhelp/firstguide/firstintro/firstintro.htm','0','거래절차안내 - 교보증권');", root.String(), "/main.jsp", "800", "/customhelp/firstguide/firstintro/firstintro.htm?scr_id=800"},
		{"javascript:maingo('1747','/common/sitemap/sitemap.jsp','0', '사이트맵 - 교보증권')", root.String(), "/main.jsp", "1747", "/common/sitemap/sitemap.jsp?scr_id=1747"},
		{"usrBody('236','/cyber/priinfo/CBPIMemberOpenAcct.jsp','1','ID등록');", root.String(), "/login_usrbody.jsp", "236", "/cyber/priinfo/CBPIMemberOpenAcct.jsp?scr_id=236"},
		{"usrBody('1402','/customhelp/page.htm?x=1','0',someTitle);", root.String(), "/main.jsp", "1402", "/customhelp/page.htm?scr_id=1402&x=1"},
		{"maingo('814','/compintro/kyobointro/CIKIsummary.htm','0','교보증권은');", "https://www.iprovest.com/compintro/index.jsp", "/compintro/default.jsp", "814", "/compintro/kyobointro/CIKIsummary.htm?scr_id=814"},
	}
	for _, tc := range cases {
		raw, ok := iprovestMenuURL(tc.handler, root, tc.source)
		if !ok {
			t.Fatalf("menu was not recognized: %q", tc.handler)
		}
		parsed := mustURL(t, raw)
		if parsed.Path != tc.page || parsed.Query().Get("scr_id") != tc.id || parsed.Query().Get("linkUrl") != tc.inner {
			t.Fatalf("wrong navigation for %q: %s", tc.handler, raw)
		}
	}
	for _, handler := range []string{
		"usrBody('1','','0','category')",
		"usrBody('1','https://external.example/page','0','external')",
		"logout()",
	} {
		if raw, ok := iprovestMenuURL(handler, root, root.String()); ok {
			t.Fatalf("unsafe menu accepted: %q => %q", handler, raw)
		}
	}
	if raw, ok := iprovestMenuURL(cases[0].handler, mustURL(t, "https://other.example/"), root.String()); ok {
		t.Fatalf("site-specific handler accepted elsewhere: %q", raw)
	}
}

func TestStoreDeduplicatesAndPreservesPaths(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, created, err := store.SaveBody("https://example.com/css/site.css", "text/css", []byte("body{}"))
	if err != nil || !created || first != "site/example.com/css/site.css" {
		t.Fatalf("first save: %q, %v, %v", first, created, err)
	}
	second, created, err := store.SaveBody("https://example.com/css/site.css", "text/css", []byte("body{}"))
	if err != nil || created || second != first {
		t.Fatalf("duplicate save: %q, %v, %v", second, created, err)
	}
	third, created, err := store.SaveBody("https://example.com/css/site.css", "text/css", []byte("body{color:red}"))
	if err != nil || !created || third == first || !strings.Contains(third, "__v") {
		t.Fatalf("version save: %q, %v, %v", third, created, err)
	}
	if _, err := os.Stat(filepath.Join(store.Root, filepath.FromSlash(third))); err != nil {
		t.Fatal(err)
	}
}

func TestCrawlLocalSite(t *testing.T) {
	chrome := DefaultChromePath()
	if chrome == "" {
		t.Skip("Chrome is not installed")
	}
	var mu sync.Mutex
	var hits []struct {
		path string
		at   time.Time
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/shared.css" {
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, "body{color:#123}")
			return
		}
		if r.URL.Path == "/delayed.json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"delayed":true}`)
			return
		}
		mu.Lock()
		hits = append(hits, struct {
			path string
			at   time.Time
		}{r.URL.Path, time.Now()})
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="stylesheet" href="/shared.css"></head><body><nav><ul><li><h2>Section</h2><a href="/a">Alpha</a><a href="/b">Beta</a><a href="/a#section">Alpha duplicate</a><a href="https://other.example/no">External</a></li></ul></nav><main>%s</main><script>setTimeout(() => fetch('/delayed.json'), 500)</script></body></html>`, r.URL.Path)
	}))
	defer server.Close()
	manager := NewManager(chrome, filepath.Join(t.TempDir(), "profile"), false)
	defer manager.Close()
	_, err := manager.Start(Config{URL: server.URL, OutputDir: t.TempDir(), MinDelayMS: 1000, IdleMS: 1000, MaxWaitMS: 10000, MaxMenus: 10})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(40 * time.Second)
	for {
		status := manager.Status()
		if status.State == "complete" {
			break
		}
		if status.State == "error" {
			t.Fatal(status.Message)
		}
		select {
		case <-deadline:
			t.Fatalf("crawl timed out: %+v", status)
		case <-time.After(100 * time.Millisecond):
		}
	}
	report := manager.Report()
	if report == nil || len(report.Pages) != 3 {
		t.Fatalf("expected three pages, got %+v", report)
	}
	if len(report.Menus) != 3 {
		t.Fatalf("expected three graph menus, got %d", len(report.Menus))
	}
	if path := report.Menus[1].Path; len(path) != 2 || path[0] != "Section" || path[1] != "Alpha" {
		t.Fatalf("menu hierarchy was lost: %v", path)
	}
	foundCSS := false
	foundDelayed := false
	for _, page := range report.Pages {
		for _, resource := range page.Resources {
			if strings.HasSuffix(resource.Path, "/shared.css") {
				foundCSS = true
			}
			if strings.HasSuffix(resource.Path, "/delayed.json") {
				foundDelayed = true
			}
		}
	}
	if !foundCSS {
		t.Fatalf("shared stylesheet was not saved: %+v", report.Pages)
	}
	if !foundDelayed {
		t.Fatalf("delayed resource was not saved: %+v", report.Pages)
	}
	mu.Lock()
	recorded := append([]struct {
		path string
		at   time.Time
	}(nil), hits...)
	mu.Unlock()
	if len(recorded) != 3 {
		t.Fatalf("expected three menu requests, got %+v", recorded)
	}
	for i := 1; i < len(recorded); i++ {
		if recorded[i].at.Sub(recorded[i-1].at) < 950*time.Millisecond {
			t.Fatalf("menu requests too close: %+v", recorded)
		}
	}
	if _, err := os.Stat(filepath.Join(report.OutputDir, "site")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(report.OutputDir, "menus", "Section", "Alpha__"+shortHash(server.URL+"/a"), "page.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(report.ZIPPath); err != nil {
		t.Fatal(err)
	}
}
