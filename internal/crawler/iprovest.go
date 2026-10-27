package crawler

import (
	"net/url"
	"strings"
)

// iprovestMenuURL turns the site's static menu callbacks into read-only page
// URLs. It never executes the callback or submits the site's hidden form.
func iprovestMenuURL(handler string, root *url.URL, sourceURL string) (string, bool) {
	if root == nil || !strings.EqualFold(root.Hostname(), "www.iprovest.com") {
		return "", false
	}
	name, args, ok := literalMenuCall(handler)
	if !ok || (name != "usrBody" && name != "maingo") || len(args) < 3 {
		return "", false
	}
	screenID, rawTarget, login := args[0], args[1], args[2]
	if screenID == "" || rawTarget == "" || (login != "0" && login != "1") {
		return "", false
	}
	for _, r := range screenID {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	target, err := url.Parse(rawTarget)
	if err != nil || target.IsAbs() || strings.HasPrefix(rawTarget, "//") || target.Fragment != "" {
		return "", false
	}
	base := *root
	base.Path = "/"
	base.RawQuery = ""
	base.Fragment = ""
	target = base.ResolveReference(target)
	if target.Host != root.Host || target.Scheme != root.Scheme || target.Path == "/" {
		return "", false
	}
	innerQuery := target.Query()
	innerQuery.Set("scr_id", screenID)
	target.RawQuery = innerQuery.Encode()
	page := "/main.jsp"
	companyPage := false
	if source, err := url.Parse(sourceURL); err == nil && source.Host == root.Host && strings.HasPrefix(source.Path, "/compintro/") {
		page = "/compintro/default.jsp"
		companyPage = true
	} else if login == "1" {
		page = "/login_usrbody.jsp"
	}
	outer := url.URL{Scheme: root.Scheme, Host: root.Host, Path: page}
	query := url.Values{}
	if !companyPage {
		query.Set("amho_yn", login)
		query.Set("idx", "")
	}
	query.Set("linkUrl", target.RequestURI())
	query.Set("scr_id", screenID)
	outer.RawQuery = query.Encode()
	return outer.String(), true
}

// literalMenuCall accepts only a plain call with quoted string arguments.
// Parsing data instead of evaluating JavaScript avoids running arbitrary page code.
func literalMenuCall(raw string) (string, []string, bool) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(s), "javascript:") {
		s = strings.TrimSpace(s[len("javascript:"):])
	}
	if strings.HasPrefix(strings.ToLower(s), "return ") {
		s = strings.TrimSpace(s[len("return "):])
	}
	open := strings.IndexByte(s, '(')
	if open < 0 {
		return "", nil, false
	}
	name := strings.TrimSpace(s[:open])
	s = s[open+1:]
	args := []string{}
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return "", nil, false
		}
		if s[0] == ')' {
			return name, args, true
		}
		quote := s[0]
		if quote != '\'' && quote != '"' {
			return "", nil, false
		}
		s = s[1:]
		var value strings.Builder
		closed := false
		for len(s) > 0 {
			ch := s[0]
			s = s[1:]
			if ch == quote {
				closed = true
				break
			}
			if ch == '\\' && len(s) > 0 {
				ch = s[0]
				s = s[1:]
			}
			value.WriteByte(ch)
		}
		if !closed {
			return "", nil, false
		}
		args = append(args, value.String())
		if len(args) == 3 {
			return name, args, true
		}
		s = strings.TrimLeft(s, " \t\r\n")
		if len(s) > 0 && s[0] == ',' {
			s = s[1:]
			continue
		}
		if len(s) > 0 && s[0] == ')' {
			return name, args, true
		}
		return "", nil, false
	}
}
