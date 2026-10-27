package crawler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LoadReport opens a capture folder or the newest capture below an output folder.
func LoadReport(directory string) (*Report, error) {
	if directory == "" {
		return nil, errors.New("저장 폴더를 지정하세요")
	}
	selected := filepath.Join(directory, "capture-report.json")
	if _, err := os.Stat(selected); err != nil {
		entries, readErr := os.ReadDir(directory)
		if readErr != nil {
			return nil, readErr
		}
		latest := ""
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "capture-") && entry.Name() > latest {
				candidate := filepath.Join(directory, entry.Name(), "capture-report.json")
				if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
					latest = entry.Name()
				}
			}
		}
		if latest == "" {
			return nil, errors.New("저장 폴더에서 수집 보고서를 찾지 못했습니다")
		}
		selected = filepath.Join(directory, latest, "capture-report.json")
	}
	info, err := os.Stat(selected)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 256*1024*1024 {
		return nil, errors.New("보고서 파일이 아니거나 256 MiB를 초과합니다")
	}
	data, err := os.ReadFile(selected)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	if report.RootURL == "" || report.Pages == nil {
		return nil, errors.New("V.W.R.D 수집 보고서가 아닙니다")
	}
	return &report, nil
}
