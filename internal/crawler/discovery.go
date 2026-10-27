package crawler

import (
	"encoding/json"
	"fmt"
)

type discoveredLink struct {
	URL     string   `json:"url"`
	Handler string   `json:"handler,omitempty"`
	Label   string   `json:"label"`
	Path    []string `json:"path"`
	Order   int      `json:"order"`
}

type menuSnapshot struct {
	URL     string           `json:"url"`
	Title   string           `json:"title"`
	Links   []discoveredLink `json:"links"`
	Skipped []string         `json:"skipped"`
	Error   string           `json:"error"`
}

func menuDiscoveryScript(selector string) string {
	value, _ := json.Marshal(selector)
	return fmt.Sprintf(`(() => {
  const custom = %s;
  const clean = (s, limit = 100) => (s || '').replace(/\s+/g, ' ').trim().slice(0, limit);
  let roots = [];
  try {
    roots = custom
      ? [...document.querySelectorAll(custom)]
      : [...document.querySelectorAll('nav, [role="navigation"], [role="menu"], [role="menubar"], aside, [class*="menu" i], [id*="menu" i], [class*="nav" i], [id*="nav" i], [class*="gnb" i], [class*="lnb" i], [class*="snb" i]')];
  } catch (error) {
    return {url: location.href, title: document.title, links: [], skipped: [], error: String(error)};
  }
  const itemSelector = 'a[href], a[onclick], button[onclick], [role="menuitem"][onclick], [data-url], [data-href], [data-route], [routerlink]';
  roots = roots.filter(el => el.querySelectorAll(itemSelector).length >= 2);
  if (!roots.length && !custom) roots = [...document.querySelectorAll('header')].filter(el => el.querySelectorAll(itemSelector).length >= 2);
  const seen = new Set(), links = [], skipped = [];
  function label(el) { return clean(el?.getAttribute('aria-label') || el?.getAttribute('title') || el?.innerText || el?.textContent); }
  function hierarchy(anchor, root) {
    const path = [];
    let node = anchor.parentElement;
    while (node && node !== root && path.length < 8) {
      if (node.matches('li, [role="group"], [role="menuitem"]')) {
        const direct = [...node.children].find(child => child.matches('a, button, [role="menuitem"], h1, h2, h3, h4, span, strong, label'));
        const name = label(direct);
        if (name && name !== label(anchor)) path.unshift(name);
      }
      if (node.previousElementSibling?.matches('h1, h2, h3, h4')) {
        const name = label(node.previousElementSibling);
        if (name && name !== label(anchor)) path.unshift(name);
      }
      node = node.parentElement;
    }
    path.push(label(anchor) || clean(anchor.getAttribute('href')));
    return path.filter((value, index) => value && value !== path[index - 1]);
  }
  for (const root of roots) {
    for (const anchor of root.querySelectorAll(itemSelector)) {
      if (seen.has(anchor)) continue;
      seen.add(anchor);
      if (links.length >= 1500) break;
      const raw = clean(anchor.getAttribute('href') || anchor.getAttribute('data-url') || anchor.getAttribute('data-href') || anchor.getAttribute('data-route') || anchor.getAttribute('routerlink'), 2048);
      const handler = clean(anchor.getAttribute('onclick') || (/^javascript:/i.test(raw) ? raw : ''), 2048);
      const name = label(anchor);
      if (anchor.hasAttribute('download')) continue;
      if (!raw || raw === '#' || /^javascript:/i.test(raw)) {
        if (handler) links.push({url: '', handler, label: name, path: hierarchy(anchor, root), order: links.length});
        else if (name) skipped.push(name);
        continue;
      }
      let target;
      try { target = new URL(raw, location.href); } catch { continue; }
      if (!/^https?:$/.test(target.protocol)) continue;
      links.push({url: target.href, label: name || target.pathname, path: hierarchy(anchor, root), order: links.length});
    }
  }
  return {url: location.href, title: document.title, links, skipped, error: ''};
})()`, string(value))
}
