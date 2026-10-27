package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type responseMeta struct {
	URL       string
	MIME      string
	Status    int64
	Capture   *pageCapture
	Responded bool
	Finished  bool
}

type pageCapture struct {
	mu        sync.Mutex
	store     *Store
	menuURL   string
	resources []Resource
	seen      map[string]struct{}
	inflight  map[network.RequestID]struct{}
	pending   int
	last      time.Time
	active    bool
}

func newPageCapture(store *Store, menuURL string) *pageCapture {
	return &pageCapture{store: store, menuURL: menuURL, seen: make(map[string]struct{}), inflight: make(map[network.RequestID]struct{}), last: time.Now(), active: true}
}

func (c *pageCapture) touch() {
	c.mu.Lock()
	if c.active {
		c.last = time.Now()
	}
	c.mu.Unlock()
}

func (c *pageCapture) requestStarted(id network.RequestID) {
	c.mu.Lock()
	if c.active {
		c.inflight[id] = struct{}{}
		c.last = time.Now()
	}
	c.mu.Unlock()
}

func (c *pageCapture) requestFinished(id network.RequestID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflight, id)
	if !c.active {
		return false
	}
	c.pending++
	c.last = time.Now()
	return true
}

func (c *pageCapture) addResource(resource Resource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.active {
		return
	}
	identity := resource.URL + "|" + resource.Path + "|" + resource.Error
	if _, ok := c.seen[identity]; !ok {
		c.seen[identity] = struct{}{}
		resource.FromMenu = c.menuURL
		c.resources = append(c.resources, resource)
	}
	c.last = time.Now()
}

func (c *pageCapture) bodyDone() {
	c.mu.Lock()
	if c.pending > 0 {
		c.pending--
	}
	if c.active {
		c.last = time.Now()
	}
	c.mu.Unlock()
}

func (c *pageCapture) waitIdle(ctx context.Context, idle, maxWait time.Duration) string {
	start := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		c.mu.Lock()
		quiet := time.Since(c.last)
		pending := c.pending
		inflight := len(c.inflight)
		c.mu.Unlock()
		if time.Since(start) >= time.Second && quiet >= idle && pending == 0 && inflight == 0 {
			return "idle"
		}
		if time.Since(start) >= maxWait {
			return "timeout"
		}
		select {
		case <-ctx.Done():
			return "cancelled"
		case <-ticker.C:
		}
	}
}

func (c *pageCapture) close() []Resource {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	return append([]Resource(nil), c.resources...)
}

type Browser struct {
	mu               sync.Mutex
	openMu           sync.Mutex
	ctx              context.Context
	cancelTab        context.CancelFunc
	cancelAllocator  context.CancelFunc
	current          *pageCapture
	requests         map[network.RequestID]responseMeta
	completed        map[network.RequestID]struct{}
	lastVisitStarted time.Time
	chromePath       string
	profileDir       string
	visible          bool
}

func NewBrowser(chromePath, profileDir string, visible bool) *Browser {
	return &Browser{chromePath: chromePath, profileDir: profileDir, visible: visible, requests: make(map[network.RequestID]responseMeta), completed: make(map[network.RequestID]struct{})}
}

func (b *Browser) Open(url string) error {
	b.openMu.Lock()
	defer b.openMu.Unlock()
	b.mu.Lock()
	if b.ctx != nil {
		ctx := b.ctx
		b.mu.Unlock()
		if url == "" {
			return nil
		}
		return chromedp.Do(ctx, chromedp.Navigate(url))
	}
	b.mu.Unlock()
	if b.chromePath == "" {
		return errors.New("Chrome 실행 파일 경로가 설정되지 않았습니다")
	}
	if _, err := os.Stat(b.chromePath); err != nil {
		return fmt.Errorf("Chrome 실행 파일: %w", err)
	}
	if b.profileDir == "" {
		return errors.New("브라우저 프로필 경로가 비어 있습니다")
	}
	if err := os.MkdirAll(b.profileDir, 0700); err != nil {
		return err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(b.chromePath), chromedp.UserDataDir(b.profileDir), chromedp.Flag("no-first-run", true), chromedp.Flag("no-default-browser-check", true))
	if b.visible {
		opts = append(opts, chromedp.VisibleWindow)
	}
	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocatorCtx)
	if err := chromedp.Do(tabCtx); err != nil {
		tabCancel()
		allocatorCancel()
		return err
	}
	if _, err := chromedp.Call(tabCtx, network.Enable, network.EnableParams{}); err != nil {
		tabCancel()
		allocatorCancel()
		return err
	}
	b.mu.Lock()
	b.ctx = tabCtx
	b.cancelTab = tabCancel
	b.cancelAllocator = allocatorCancel
	b.mu.Unlock()
	b.startEvents(tabCtx)
	if url != "" {
		return chromedp.Do(tabCtx, chromedp.Navigate(url))
	}
	return nil
}

func (b *Browser) Close() {
	b.openMu.Lock()
	defer b.openMu.Unlock()
	b.mu.Lock()
	cancelTab := b.cancelTab
	cancelAllocator := b.cancelAllocator
	b.ctx = nil
	b.cancelTab = nil
	b.cancelAllocator = nil
	b.current = nil
	b.requests = make(map[network.RequestID]responseMeta)
	b.completed = make(map[network.RequestID]struct{})
	b.mu.Unlock()
	if cancelTab != nil {
		cancelTab()
	}
	if cancelAllocator != nil {
		cancelAllocator()
	}
}

func (b *Browser) startEvents(ctx context.Context) {
	started := chromedp.Events(ctx, network.RequestWillBeSent)
	responses := chromedp.Events(ctx, network.ResponseReceived)
	finished := chromedp.Events(ctx, network.LoadingFinished)
	failed := chromedp.Events(ctx, network.LoadingFailed)
	go func() {
		for ev, err := range started {
			if err != nil {
				return
			}
			b.mu.Lock()
			if _, done := b.completed[ev.RequestID]; done {
				b.mu.Unlock()
				continue
			}
			capture := b.current
			if capture != nil {
				meta := b.requests[ev.RequestID]
				if meta.Capture == nil {
					meta.Capture = capture
				}
				if !meta.Responded {
					meta.URL = ev.Request.URL
				}
				if !meta.Finished {
					capture.requestStarted(ev.RequestID)
				}
				b.requests[ev.RequestID] = meta
			}
			b.mu.Unlock()
		}
	}()
	go func() {
		for ev, err := range responses {
			if err != nil {
				return
			}
			b.mu.Lock()
			if _, done := b.completed[ev.RequestID]; done {
				b.mu.Unlock()
				continue
			}
			meta, ok := b.requests[ev.RequestID]
			if !ok && b.current != nil {
				meta.Capture = b.current
			}
			fetch := false
			if meta.Capture != nil && ev.Response != nil {
				meta.URL = ev.Response.URL
				meta.MIME = ev.Response.MimeType
				meta.Status = ev.Response.Status
				meta.Responded = true
				if meta.Finished {
					delete(b.requests, ev.RequestID)
					b.completed[ev.RequestID] = struct{}{}
					fetch = true
				} else {
					b.requests[ev.RequestID] = meta
				}
				meta.Capture.touch()
			}
			b.mu.Unlock()
			if fetch {
				go b.fetchBody(ctx, ev.RequestID, meta)
			}
		}
	}()
	go func() {
		for ev, err := range finished {
			if err != nil {
				return
			}
			b.mu.Lock()
			if _, done := b.completed[ev.RequestID]; done {
				b.mu.Unlock()
				continue
			}
			meta, ok := b.requests[ev.RequestID]
			if !ok && b.current != nil {
				meta.Capture = b.current
				ok = true
			}
			fetch := false
			if ok && meta.Capture != nil && !meta.Finished {
				meta.Finished = true
				if meta.Capture.requestFinished(ev.RequestID) {
					if meta.Responded {
						delete(b.requests, ev.RequestID)
						b.completed[ev.RequestID] = struct{}{}
						fetch = true
					} else {
						b.requests[ev.RequestID] = meta
					}
				}
			}
			b.mu.Unlock()
			if fetch {
				go b.fetchBody(ctx, ev.RequestID, meta)
			}
		}
	}()
	go func() {
		for ev, err := range failed {
			if err != nil {
				return
			}
			b.mu.Lock()
			meta, ok := b.requests[ev.RequestID]
			delete(b.requests, ev.RequestID)
			b.completed[ev.RequestID] = struct{}{}
			b.mu.Unlock()
			if !ok || meta.Capture == nil {
				continue
			}
			meta.Capture.mu.Lock()
			delete(meta.Capture.inflight, ev.RequestID)
			meta.Capture.mu.Unlock()
			meta.Capture.addResource(Resource{URL: meta.URL, MIME: meta.MIME, Status: meta.Status, Error: ev.ErrorText})
		}
	}()
}

func (b *Browser) fetchBody(ctx context.Context, id network.RequestID, meta responseMeta) {
	defer meta.Capture.bodyDone()
	if meta.URL == "" {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := chromedp.Call(readCtx, network.GetResponseBody, network.GetResponseBodyParams{RequestID: id})
	if err != nil {
		meta.Capture.addResource(Resource{URL: meta.URL, MIME: meta.MIME, Status: meta.Status, Error: err.Error()})
		return
	}
	if len(result.Body) > 250*1024*1024 {
		meta.Capture.addResource(Resource{URL: meta.URL, MIME: meta.MIME, Status: meta.Status, Error: "response body exceeds 250 MiB limit"})
		return
	}
	path, _, err := meta.Capture.store.SaveBody(meta.URL, meta.MIME, result.Body)
	if err != nil {
		meta.Capture.addResource(Resource{URL: meta.URL, MIME: meta.MIME, Status: meta.Status, Error: err.Error()})
		return
	}
	meta.Capture.addResource(Resource{URL: meta.URL, Path: path, MIME: meta.MIME, Status: meta.Status, Bytes: len(result.Body)})
}

func (b *Browser) Visit(ctx context.Context, menu Menu, config Config, store *Store) (Page, menuSnapshot) {
	page := Page{Menu: menu, VisitedAt: time.Now(), Resources: []Resource{}}
	snapshot := menuSnapshot{}
	b.mu.Lock()
	baseCtx := b.ctx
	if baseCtx == nil {
		b.mu.Unlock()
		page.Error = "browser is not open"
		return page, snapshot
	}
	last := b.lastVisitStarted
	b.mu.Unlock()
	if wait := config.Delay() - time.Since(last); !last.IsZero() && wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			page.Error = ctx.Err().Error()
			return page, snapshot
		case <-timer.C:
		}
	}
	capture := newPageCapture(store, menu.URL)
	b.mu.Lock()
	b.current = capture
	b.lastVisitStarted = time.Now()
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if b.current == capture {
			b.current = nil
		}
		b.mu.Unlock()
	}()
	visitCtx, cancel := context.WithTimeout(ctx, config.MaxWait())
	navErr := chromedp.Do(visitCtx, chromedp.Navigate(menu.URL))
	cancel()
	if navErr != nil && !errors.Is(navErr, context.DeadlineExceeded) {
		page.Error = navErr.Error()
	}
	capture.touch()
	page.WaitResult = capture.waitIdle(ctx, config.Idle(), config.MaxWait())
	if page.WaitResult == "cancelled" {
		page.Error = ctx.Err().Error()
		page.Resources = capture.close()
		return page, snapshot
	}
	inspectCtx, inspectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer inspectCancel()
	if value, err := chromedp.Run(inspectCtx, chromedp.Location()); err == nil {
		page.FinalURL = value
	}
	if value, err := chromedp.Run(inspectCtx, chromedp.Title()); err == nil {
		page.Title = value
	}
	if html, err := chromedp.Run(inspectCtx, chromedp.Evaluate[string]("document.documentElement.outerHTML")); err == nil && html != "" {
		urlForHTML := page.FinalURL
		if urlForHTML == "" {
			urlForHTML = menu.URL
		}
		if path, _, err := store.SaveBody(urlForHTML, "text/html", []byte(html)); err == nil {
			page.HTMLPath = path
		}
	}
	if value, err := chromedp.Run(inspectCtx, chromedp.Evaluate[menuSnapshot](menuDiscoveryScript(config.MenuSelector))); err == nil {
		snapshot = value
	} else if page.Error == "" {
		page.Error = "menu discovery: " + err.Error()
	}
	page.Resources = capture.close()
	if page.FinalURL == "" {
		page.FinalURL = menu.URL
	}
	return page, snapshot
}

func DefaultChromePath() string {
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}
	for _, item := range candidates {
		if _, err := os.Stat(item); err == nil {
			return item
		}
	}
	return ""
}

func DefaultProfileDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "SaveAllResourcesGo", "ChromeProfile")
}

func isNavigableMenu(raw string, base *url.URL) bool {
	_, _, ok := canonicalMenuURL(raw, base)
	return ok && !strings.Contains(strings.ToLower(raw), "logout") && !strings.Contains(raw, "로그아웃")
}
