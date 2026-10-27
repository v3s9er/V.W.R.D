package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu      sync.Mutex
	browser *Browser
	status  Status
	report  *Report
	cancel  context.CancelFunc
}

func NewManager(chromePath, profileDir string, visible bool) *Manager {
	return &Manager{browser: NewBrowser(chromePath, profileDir, visible), status: Status{State: "idle", Message: "준비되었습니다"}}
}

func (m *Manager) OpenBrowser(rawURL string) error {
	m.mu.Lock()
	if m.status.State == "running" || m.status.State == "starting" {
		m.mu.Unlock()
		return errors.New("수집 중에는 브라우저를 다시 열 수 없습니다")
	}
	m.mu.Unlock()
	if rawURL != "" {
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("HTTP(S) 사이트 주소를 입력하세요")
		}
		rawURL = u.String()
	}
	if err := m.browser.Open(rawURL); err != nil {
		return err
	}
	m.setStatus(func(s *Status) {
		s.Message = "브라우저가 열렸습니다. 필요한 경우 로그인한 뒤 수집을 시작하세요"
	})
	return nil
}

func (m *Manager) Start(config Config) (string, error) {
	if err := config.Normalize(); err != nil {
		return "", err
	}
	m.mu.Lock()
	if m.status.State == "running" || m.status.State == "starting" {
		m.mu.Unlock()
		return "", errors.New("이미 수집 중입니다")
	}
	m.status.State = "starting"
	m.status.Message = "브라우저를 준비하는 중입니다"
	m.mu.Unlock()
	if err := m.browser.Open(""); err != nil {
		m.finishWithError(err)
		return "", err
	}
	m.browser.mu.Lock()
	base := m.browser.ctx
	m.browser.mu.Unlock()
	ctx, cancel := context.WithCancel(base)
	jobID := time.Now().Format("20060102-150405.000000000")
	m.mu.Lock()
	m.cancel = cancel
	m.report = nil
	m.status = Status{State: "running", CurrentURL: config.URL, Message: "메뉴를 탐색하는 중입니다", StartedAt: time.Now()}
	m.mu.Unlock()
	go m.run(ctx, config)
	return jobID, nil
}

func (m *Manager) run(ctx context.Context, config Config) {
	store, err := NewStore(config.OutputDir)
	if err != nil {
		m.finishWithError(err)
		return
	}
	root, _ := url.Parse(config.URL)
	_, rootKey, _ := canonicalMenuURL(config.URL, root)
	queue := []Menu{{URL: config.URL, Label: "Home", Path: []string{"Home"}, Order: 0, Key: rootKey}}
	seen := map[string]struct{}{rootKey: {}}
	report := Report{RootURL: config.URL, StartedAt: time.Now(), DelayMS: config.MinDelayMS, Pages: []Page{}, OutputDir: store.Root, ZIPPath: store.Root + ".zip"}
	for index := 0; index < len(queue) && index < config.MaxMenus; index++ {
		if ctx.Err() != nil {
			break
		}
		menu := queue[index]
		m.setStatus(func(s *Status) {
			s.CurrentURL = menu.URL
			s.Discovered = len(queue)
			s.Message = fmt.Sprintf("%d/%d 메뉴 방문 중: %s", index+1, len(queue), menu.Label)
		})
		page, snapshot := m.browser.Visit(ctx, menu, config, store)
		if snapshot.Error != "" && page.Error == "" {
			page.Error = "menu discovery: " + snapshot.Error
		}
		if _, err := store.SavePage(page); err != nil && page.Error == "" {
			page.Error = "save page: " + err.Error()
		}
		report.Pages = append(report.Pages, page)
		if _, finalKey, ok := canonicalMenuURL(page.FinalURL, root); ok {
			seen[finalKey] = struct{}{}
		}
		report.Skipped = append(report.Skipped, snapshot.Skipped...)
		m.setStatus(func(s *Status) { s.Visited = len(report.Pages); s.Saved = store.Count(); s.ResultDir = store.Root })
		for _, link := range snapshot.Links {
			candidate := link.URL
			if candidate == "" && link.Handler != "" {
				candidate, _ = iprovestMenuURL(link.Handler, root, snapshot.URL)
			}
			if candidate == "" {
				if link.Label != "" {
					report.Skipped = append(report.Skipped, link.Label)
				}
				continue
			}
			if !isNavigableMenu(candidate, root) || strings.Contains(strings.ToLower(link.Label), "logout") || strings.Contains(link.Label, "로그아웃") {
				continue
			}
			actual, key, ok := canonicalMenuURL(candidate, root)
			if !ok {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			label := strings.TrimSpace(link.Label)
			if label == "" {
				label = actual
			}
			path := link.Path
			if len(path) == 0 {
				path = append(append([]string(nil), menu.Path...), label)
			}
			queue = append(queue, Menu{URL: actual, Label: label, Path: path, ParentURL: menu.URL, Order: link.Order, Key: key})
		}
		m.setStatus(func(s *Status) { s.Discovered = len(queue) })
	}
	report.FinishedAt = time.Now()
	report.Menus = append([]Menu(nil), queue...)
	if err := store.SaveReport(report); err != nil {
		m.finishWithError(err)
		return
	}
	zipPath, err := store.Zip()
	if err != nil {
		m.finishWithError(err)
		return
	}
	report.ZIPPath = zipPath
	m.mu.Lock()
	m.report = &report
	if ctx.Err() != nil {
		m.status.State = "stopped"
		m.status.Message = "중지되었습니다. 완료된 메뉴의 결과를 저장했습니다"
	} else {
		m.status.State = "complete"
		m.status.Message = fmt.Sprintf("완료: 메뉴 %d개 방문", len(report.Pages))
	}
	m.status.ZIPPath = zipPath
	m.status.ResultDir = store.Root
	m.status.FinishedAt = time.Now()
	m.cancel = nil
	m.mu.Unlock()
}

func (m *Manager) finishWithError(err error) {
	m.mu.Lock()
	m.status.State = "error"
	m.status.Message = err.Error()
	m.status.FinishedAt = time.Now()
	m.cancel = nil
	m.mu.Unlock()
}

func (m *Manager) setStatus(change func(*Status)) {
	m.mu.Lock()
	change(&m.status)
	m.mu.Unlock()
}

func (m *Manager) Stop() bool {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) Report() *Report {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.report
}

func (m *Manager) Close() {
	m.Stop()
	m.browser.Close()
}
