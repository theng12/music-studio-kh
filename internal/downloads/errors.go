package downloads

import (
	"strings"
)

func cleanToolError(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch {
	case strings.Contains(s, "Encoder not found"):
		return "Cover conversion failed because a bundled encoder is unavailable. Update or reinstall Music Studio KH."
	case strings.Contains(s, "No space left on device") || strings.Contains(s, "not enough free disk space"):
		return "There is not enough free disk space in the music folder. Free some space, then retry."
	case strings.Contains(s, "This video is unavailable") || strings.Contains(s, "Video unavailable") || strings.Contains(s, "does not exist"):
		return "This video is unavailable. Check that the link opens on YouTube, then retry."
	case strings.Contains(s, "Private video"):
		return "This video is private. Check video permissions on YouTube, then retry."
	case strings.Contains(s, "confirm your age") || strings.Contains(s, "Sign in to confirm") || strings.Contains(s, "Sign in if you've been granted access"):
		return "This video requires sign-in or age confirmation and cannot be downloaded."
	case strings.Contains(s, "Members-only") || strings.Contains(s, "members-only") || strings.Contains(s, "Join this channel"):
		return "This video is available to channel members only."
	case strings.Contains(s, "not available in your country") || strings.Contains(s, "not available in your region") || strings.Contains(s, "blocked in your country"):
		return "This video is not available in your region."
	case strings.Contains(s, "Premieres in") || strings.Contains(s, "premieres in"):
		return "This video has not premiered yet."
	case strings.Contains(s, "Unable to download webpage") || strings.Contains(s, "urlopen error") ||
		strings.Contains(s, "nodename nor servname provided") || strings.Contains(s, "Name or service not known") ||
		strings.Contains(s, "Connection refused") || strings.Contains(s, "Network is unreachable") ||
		strings.Contains(s, "timed out"):
		return "Network connection failed. Check your internet connection, then retry."
	case strings.Contains(s, "max-filesize") || strings.Contains(s, "larger than max-filesize"):
		return "Audio file exceeds the maximum allowed download size (512 MB)."
	}

	if strings.HasPrefix(s, "ERROR:") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "ERROR:"))
	}
	if strings.HasPrefix(s, "[") {
		if idx := strings.Index(s, "]: "); idx != -1 {
			s = strings.TrimSpace(s[idx+3:])
		}
	}

	// Never leak local filesystem paths or usernames
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return ""
	}
	// Tool diagnostics must never expose authentication material.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "password") || strings.Contains(lower, "cookie") || strings.Contains(lower, "bearer") || strings.Contains(lower, "authorization") {
		return ""
	}
	if strings.Contains(s, "?") || strings.Contains(lower, "token=") || strings.Contains(lower, "auth=") || strings.Contains(lower, "key=") {
		return ""
	}
	for _, ch := range s {
		if ch < 32 || ch > 126 {
			return ""
		}
	}
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func sanitizeJobError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch msg {
	case "no downloadable audio found":
		return "No downloadable audio found for this link."
	case "download output was outside staging":
		return "Download output was invalid or outside staging."
	case "download output is not an audio file":
		return "Download output is not a supported audio file."
	case "playlist is empty or too large":
		return "Playlist is empty or exceeds the 1,000 track limit."
	case "playlist resolver unavailable":
		return "Playlist resolver unavailable. Check app installation and tools."
	case "download tools unavailable":
		return "Download tools unavailable. Reinstall Music Studio KH."
	case "could not start download tool":
		return "Could not start download tool. Check app permissions."
	case "download tool failed":
		return "Download could not finish. Check the source, connection, available storage, and music folder, then retry."
	}
	if cleaned := cleanToolError(msg); cleaned != "" {
		return cleaned
	}
	return "Download could not finish. Check the source, connection, available storage, and music folder, then retry."
}
