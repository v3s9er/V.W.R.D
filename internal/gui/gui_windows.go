package gui

import (
	"context"
	"embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/v3s9er/V.W.R.D/internal/crawler"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/*
var assets embed.FS

type Bridge struct {
	manager *crawler.Manager
	ctx     context.Context
}

func (b *Bridge) startup(ctx context.Context) { b.ctx = ctx }

func (b *Bridge) Defaults() crawler.Config {
	home, _ := os.UserHomeDir()
	return crawler.Config{
		OutputDir:  filepath.Join(home, "Documents", "SaveAllResources"),
		MinDelayMS: 1000,
		IdleMS:     3000,
		MaxWaitMS:  45000,
		MaxMenus:   1000,
	}
}

func (b *Bridge) ChooseDirectory(current string) (string, error) {
	if info, err := os.Stat(current); err != nil || !info.IsDir() {
		current = ""
	}
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "저장 폴더 선택", DefaultDirectory: current, CanCreateDirectories: true,
	})
}

func (b *Bridge) ChooseReportDirectory(current string) (string, error) {
	if info, err := os.Stat(current); err != nil || !info.IsDir() {
		current = ""
	}
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "분석 폴더 선택", DefaultDirectory: current,
	})
}

func (b *Bridge) OpenBrowser(address string) error            { return b.manager.OpenBrowser(address) }
func (b *Bridge) Start(config crawler.Config) (string, error) { return b.manager.Start(config) }
func (b *Bridge) Stop() bool                                  { return b.manager.Stop() }
func (b *Bridge) Status() crawler.Status                      { return b.manager.Status() }
func (b *Bridge) Report() *crawler.Report                     { return b.manager.Report() }

func (b *Bridge) OpenReport(directory string) (*crawler.Report, error) {
	if directory != "" {
		return crawler.LoadReport(directory)
	}
	if report := b.manager.Report(); report != nil {
		return report, nil
	}
	return crawler.LoadReport(directory)
}

func (b *Bridge) OpenResult(requested string) error {
	directory := requested
	if directory == "" {
		directory = b.manager.Status().ResultDir
	}
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("결과 폴더가 아닙니다")
	}
	return exec.Command("explorer.exe", directory).Start()
}

func Run(manager *crawler.Manager) error {
	bridge := &Bridge{manager: manager}
	defer manager.Close()
	return wails.Run(&options.App{
		Title: "V.W.R.D",
		Width: 1120, Height: 760,
		MinWidth: 900, MinHeight: 620,
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		Bind:             []interface{}{bridge},
		OnStartup:        bridge.startup,
	})
}
