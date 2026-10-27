package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path"
	"sort"
	"strings"
)

func canonicalMenuURL(raw string, base *url.URL) (actual, key string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return "", "", false
	}
	u = base.ResolveReference(u)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", false
	}
	if !strings.EqualFold(u.Hostname(), base.Hostname()) || u.Port() != base.Port() || u.Scheme != base.Scheme {
		return "", "", false
	}
	if u.Fragment != "" && !strings.HasPrefix(u.Fragment, "/") && !strings.HasPrefix(u.Fragment, "!") {
		u.Fragment = ""
	}
	actual = u.String()
	copyURL := *u
	copyURL.RawQuery = sortedQuery(u.Query())
	key = copyURL.String()
	return actual, key, true
}

func sortedQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	for key := range values {
		sort.Strings(values[key])
	}
	return values.Encode()
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:10]
}

func sitePath(rawURL, mime string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return path.Join("_special", shortHash(rawURL)+extensionFor(mime))
	}
	parts := []string{"site", safePart(u.Host)}
	escaped := strings.TrimPrefix(u.EscapedPath(), "/")
	if escaped != "" {
		for _, item := range strings.Split(escaped, "/") {
			unescaped, err := url.PathUnescape(item)
			if err != nil {
				unescaped = item
			}
			parts = append(parts, safePart(unescaped))
		}
	}
	if len(parts) == 2 || strings.HasSuffix(u.Path, "/") {
		parts = append(parts, "index")
	}
	last := parts[len(parts)-1]
	if !hasFileExtension(last) {
		last += extensionFor(mime)
	}
	if u.RawQuery != "" {
		dot := strings.LastIndex(last, ".")
		last = last[:dot] + "__q" + shortHash(u.RawQuery) + last[dot:]
	}
	parts[len(parts)-1] = last
	return path.Join(parts...)
}

func hasFileExtension(name string) bool {
	dot := strings.LastIndex(name, ".")
	return dot > 0 && dot < len(name)-1 && len(name)-dot <= 13
}

func extensionFor(mime string) string {
	switch strings.ToLower(strings.Split(mime, ";")[0]) {
	case "text/html":
		return ".html"
	case "text/css":
		return ".css"
	case "text/javascript", "application/javascript":
		return ".js"
	case "application/json":
		return ".json"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

func safePart(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	clean := strings.TrimRight(b.String(), ". ")
	if clean == "" || clean == "." || clean == ".." {
		clean = "_"
	}
	upper := strings.ToUpper(strings.Split(clean, ".")[0])
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" || (len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9') {
		clean = "_" + clean
	}
	runes := []rune(clean)
	if len(runes) > 90 {
		clean = string(runes[:90])
	}
	return clean
}
