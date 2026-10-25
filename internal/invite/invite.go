package invite

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	Path       = "/join/"
	TimeFormat = "2006-01-02 15:04"
)

func Link(server, token string) string {
	return strings.TrimRight(server, "/") + Path + token
}

func Parse(raw string) (server, token string, err error) {
	u, parseErr := url.Parse(strings.TrimSpace(raw))
	if parseErr != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", "", notALink(raw)
	}

	rest, ok := strings.CutPrefix(u.Path, Path)
	token = strings.Trim(rest, "/")
	if !ok || token == "" || strings.Contains(token, "/") {
		return "", "", notALink(raw)
	}
	return u.Scheme + "://" + u.Host, token, nil
}

func notALink(raw string) error {
	return fmt.Errorf("%q is not an invite link (expected https://gator.example.com%s<code>)", raw, Path)
}
