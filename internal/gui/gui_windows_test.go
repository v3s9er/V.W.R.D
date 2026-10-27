package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/v3s9er/V.W.R.D/internal/crawler"
)

func TestOpenReportFindsLatestCapture(t *testing.T) {
	root := t.TempDir()
	for _, item := range []struct{ folder, url string }{
		{"capture-20260101-100000", "https://old.example/"},
		{"capture-20260102-100000", "https://new.example/"},
	} {
		folder := filepath.Join(root, item.folder)
		if err := os.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(crawler.Report{RootURL: item.url, Pages: []crawler.Page{}, Menus: []crawler.Menu{}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "capture-report.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	bridge := Bridge{manager: crawler.NewManager("", "", false)}
	report, err := bridge.OpenReport(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.RootURL != "https://new.example/" {
		t.Fatalf("opened %s", report.RootURL)
	}
}
