package downloads

import (
	"errors"
	"net/url"
	"strings"
)

func ValidateURL(raw string) (string, error) {
	if len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\x00") {
		return "", errors.New("paste a valid YouTube video or playlist link")
	}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Port() != "" {
		return "", errors.New("only YouTube HTTP(S) links are supported")
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		u.Host = "www.youtube.com"
	case "youtu.be":
	default:
		return "", errors.New("only YouTube links are supported")
	}
	u.Scheme = "https"
	u.Fragment = ""
	q := url.Values{}
	for _, key := range []string{"v", "list"} {
		if v := u.Query().Get(key); v != "" {
			if len(v) > 200 || strings.ContainsAny(v, "/\\\x00\r\n") {
				return "", errors.New("invalid video or playlist identifier")
			}
			q.Set(key, v)
		}
	}
	u.RawQuery = q.Encode()
	if host == "youtu.be" {
		if strings.Trim(u.Path, "/") == "" {
			return "", errors.New("link needs a video identifier")
		}
	} else {
		switch {
		case u.Path == "/watch" && q.Get("v") != "":
		case u.Path == "/playlist" && q.Get("list") != "":
		case strings.HasPrefix(u.Path, "/shorts/") && len(u.Path) > 8:
		default:
			return "", errors.New("paste a video, short, or playlist link")
		}
	}
	return u.String(), nil
}
