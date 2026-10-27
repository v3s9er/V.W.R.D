package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/v3s9er/V.W.R.D/internal/crawler"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	manager := crawler.NewManager("", "", false)
	server := (&Server{Manager: manager}).newProtocolServer()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go server.Run(ctx, serverTransport)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"open_browser": false, "start_crawl": false, "get_status": false, "stop_crawl": false, "get_report": false}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing MCP tool %q", name)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_status"})
	if err != nil || result.IsError || result.StructuredContent == nil {
		t.Fatalf("get_status: %+v, %v", result, err)
	}
	dir := t.TempDir()
	fixture := crawler.Report{
		RootURL: "https://example.com/",
		Menus:   []crawler.Menu{{URL: "https://example.com/"}, {URL: "https://example.com/a"}},
		Pages:   []crawler.Page{{Menu: crawler.Menu{URL: "https://example.com/", Label: "Home"}}},
		Skipped: []string{"one", "two"},
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "capture-report.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_report", Arguments: map[string]any{"output_dir": dir}})
	if err != nil || result.IsError {
		t.Fatalf("saved get_report: %+v, %v", result, err)
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output ReportOutput
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatal(err)
	}
	if output.RootURL != fixture.RootURL || output.DiscoveredMenus != 2 || output.SkippedMenus != 2 || len(output.Pages) != 1 {
		t.Fatalf("wrong saved report summary: %+v", output)
	}
}

func TestMCPExecutableSavedReport(t *testing.T) {
	binary := os.Getenv("VWRD_MCP_BIN")
	directory := os.Getenv("VWRD_CAPTURE_DIR")
	if binary == "" || directory == "" {
		t.Skip("set VWRD_MCP_BIN and VWRD_CAPTURE_DIR for an executable smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "vwrd-smoke", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, "--mcp")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_report", Arguments: map[string]any{"output_dir": directory}})
	if err != nil || result.IsError {
		t.Fatalf("executable get_report: %+v, %v", result, err)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output ReportOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.RootURL == "" || output.DiscoveredMenus == 0 || len(output.Pages) == 0 {
		t.Fatalf("empty saved report: %+v", output)
	}
	t.Logf("saved report: site=%s discovered=%d visited=%d skipped=%d", output.RootURL, output.DiscoveredMenus, len(output.Pages), output.SkippedMenus)
}
