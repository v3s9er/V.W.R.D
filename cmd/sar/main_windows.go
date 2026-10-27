package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/v3s9er/V.W.R.D/internal/crawler"
	"github.com/v3s9er/V.W.R.D/internal/gui"
	"github.com/v3s9er/V.W.R.D/internal/mcpserver"
)

func main() {
	mcpMode := flag.Bool("mcp", false, "run the MCP server over stdio")
	flag.Parse()
	manager := crawler.NewManager(crawler.DefaultChromePath(), crawler.DefaultProfileDir(), true)
	var err error
	if *mcpMode {
		defer manager.Close()
		err = mcpserver.Run(context.Background(), manager)
	} else {
		err = gui.Run(manager)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
