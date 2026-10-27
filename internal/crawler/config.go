package crawler

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	URL          string `json:"url"`
	OutputDir    string `json:"output_dir"`
	MenuSelector string `json:"menu_selector,omitempty"`
	MinDelayMS   int    `json:"min_delay_ms"`
	IdleMS       int    `json:"idle_ms"`
	MaxWaitMS    int    `json:"max_wait_ms"`
	MaxMenus     int    `json:"max_menus"`
}

func (c *Config) Normalize() error {
	u, err := url.Parse(strings.TrimSpace(c.URL))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("HTTP(S) 사이트 주소를 입력하세요")
	}
	c.URL = u.String()
	if c.MinDelayMS < 1000 {
		c.MinDelayMS = 1000
	}
	if c.IdleMS < 1000 {
		c.IdleMS = 3000
	}
	if c.MaxWaitMS < c.IdleMS+1000 {
		c.MaxWaitMS = 45000
		if c.MaxWaitMS < c.IdleMS+1000 {
			c.MaxWaitMS = c.IdleMS + 1000
		}
	}
	if c.MaxMenus <= 0 {
		c.MaxMenus = 1000
	}
	if c.MaxMenus > 1000 {
		c.MaxMenus = 1000
	}
	if c.OutputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		c.OutputDir = filepath.Join(home, "Documents", "SaveAllResources")
	}
	absolute, err := filepath.Abs(c.OutputDir)
	if err != nil {
		return err
	}
	c.OutputDir = absolute
	return nil
}

func (c Config) Delay() time.Duration   { return time.Duration(c.MinDelayMS) * time.Millisecond }
func (c Config) Idle() time.Duration    { return time.Duration(c.IdleMS) * time.Millisecond }
func (c Config) MaxWait() time.Duration { return time.Duration(c.MaxWaitMS) * time.Millisecond }
