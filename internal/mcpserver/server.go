package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/v3s9er/V.W.R.D/internal/crawler"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct{ Manager *crawler.Manager }

type OpenBrowserInput struct {
	URL string `json:"url" jsonschema:"HTTP or HTTPS site address to open for login or inspection"`
}
type OpenBrowserOutput struct {
	Message string         `json:"message"`
	Status  crawler.Status `json:"status"`
}

type StartInput struct {
	URL          string `json:"url" jsonschema:"Site root URL to crawl"`
	OutputDir    string `json:"output_dir,omitempty" jsonschema:"Directory where the structured capture will be saved"`
	MenuSelector string `json:"menu_selector,omitempty" jsonschema:"Optional CSS selector for the site's menu container"`
	MinDelayMS   int    `json:"min_delay_ms,omitempty" jsonschema:"Minimum delay between menu navigation starts; values below 1000 become 1000"`
	IdleMS       int    `json:"idle_ms,omitempty" jsonschema:"Network quiet period in milliseconds; default 3000"`
	MaxWaitMS    int    `json:"max_wait_ms,omitempty" jsonschema:"Maximum wait per menu in milliseconds; default 45000"`
	MaxMenus     int    `json:"max_menus,omitempty" jsonschema:"Maximum unique menus to visit; default 1000, maximum 1000"`
}
type StartOutput struct {
	JobID  string         `json:"job_id"`
	Status crawler.Status `json:"status"`
}
type Empty struct{}
type StatusOutput struct {
	Status crawler.Status `json:"status"`
}
type StopOutput struct {
	Stopping bool `json:"stopping"`
}
type GetReportInput struct {
	OutputDir string `json:"output_dir,omitempty" jsonschema:"Optional saved capture directory or its parent output directory"`
}
type PageSummary struct {
	MenuURL       string   `json:"menu_url"`
	MenuLabel     string   `json:"menu_label"`
	MenuPath      []string `json:"menu_path"`
	ResourceCount int      `json:"resource_count"`
	Error         string   `json:"error,omitempty"`
}
type ReportOutput struct {
	Status          crawler.Status `json:"status"`
	RootURL         string         `json:"root_url,omitempty"`
	OutputDir       string         `json:"output_dir,omitempty"`
	ZIPPath         string         `json:"zip_path,omitempty"`
	DiscoveredMenus int            `json:"discovered_menus"`
	SkippedMenus    int            `json:"skipped_menus"`
	Pages           []PageSummary  `json:"pages,omitempty"`
}

func (s *Server) Run(ctx context.Context) error {
	if s.Manager == nil {
		return errors.New("crawler manager is nil")
	}
	server := s.newProtocolServer()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("MCP server: %w", err)
	}
	return nil
}

func (s *Server) newProtocolServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "vwrd", Version: "1.0.3"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "open_browser", Description: "Open a dedicated Chrome profile at a user-selected site so the user can log in before capture."},
		func(_ context.Context, _ *mcp.CallToolRequest, input OpenBrowserInput) (*mcp.CallToolResult, OpenBrowserOutput, error) {
			if err := s.Manager.OpenBrowser(input.URL); err != nil {
				return nil, OpenBrowserOutput{}, err
			}
			return nil, OpenBrowserOutput{Message: "Browser opened; log in if needed, then call start_crawl", Status: s.Manager.Status()}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "start_crawl", Description: "Discover same-origin navigation menus, visit each canonical menu URL once with at least a one-second interval, and save resources in the site's structure. Returns immediately with a job ID."},
		func(_ context.Context, _ *mcp.CallToolRequest, input StartInput) (*mcp.CallToolResult, StartOutput, error) {
			config := crawler.Config{URL: input.URL, OutputDir: input.OutputDir, MenuSelector: input.MenuSelector, MinDelayMS: input.MinDelayMS, IdleMS: input.IdleMS, MaxWaitMS: input.MaxWaitMS, MaxMenus: input.MaxMenus}
			jobID, err := s.Manager.Start(config)
			if err != nil {
				return nil, StartOutput{}, err
			}
			return nil, StartOutput{JobID: jobID, Status: s.Manager.Status()}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_status", Description: "Get the current crawl state, visited and discovered menu counts, and output paths."},
		func(_ context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, StatusOutput, error) {
			return nil, StatusOutput{Status: s.Manager.Status()}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "stop_crawl", Description: "Stop the current crawl after its active work and save the completed menus."},
		func(_ context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, StopOutput, error) {
			return nil, StopOutput{Stopping: s.Manager.Stop()}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_report", Description: "Read the last finished crawl or a saved capture by output_dir, including menu hierarchy and file counts."},
		func(_ context.Context, _ *mcp.CallToolRequest, input GetReportInput) (*mcp.CallToolResult, ReportOutput, error) {
			output := ReportOutput{Status: s.Manager.Status()}
			report := s.Manager.Report()
			if input.OutputDir != "" {
				var err error
				report, err = crawler.LoadReport(input.OutputDir)
				if err != nil {
					return nil, output, err
				}
			}
			if report == nil {
				return nil, output, nil
			}
			output.RootURL = report.RootURL
			output.OutputDir = report.OutputDir
			output.ZIPPath = report.ZIPPath
			output.DiscoveredMenus = len(report.Menus)
			output.SkippedMenus = len(report.Skipped)
			for _, page := range report.Pages {
				output.Pages = append(output.Pages, PageSummary{MenuURL: page.Menu.URL, MenuLabel: page.Menu.Label, MenuPath: page.Menu.Path, ResourceCount: len(page.Resources), Error: page.Error})
			}
			return nil, output, nil
		})
	return server
}

func Run(ctx context.Context, manager *crawler.Manager) error {
	log.SetOutput(log.Writer()) // MCP protocol uses stdout; diagnostics remain on stderr.
	return (&Server{Manager: manager}).Run(ctx)
}
