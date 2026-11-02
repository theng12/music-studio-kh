package library

import (
	"regexp"
	"strings"
)

var artistMarker = regexp.MustCompile(`(?i)\s+(?:feat\.?|ft\.?|featuring|w/|with|x)\s+`)
var featuredSeparator = regexp.MustCompile(`\s+&\s+`)

// ArtistNames groups credits without changing the original tags. Bare ampersands
// stay intact because they can belong to a band name.
func ArtistNames(credit, albumArtist string) []string {
	parts := artistMarker.Split(strings.TrimSpace(credit), -1)
	main := strings.TrimSpace(albumArtist)
	if main == "" {
		main = parts[0]
	} else {
		main = artistMarker.Split(main, -1)[0]
	}
	names := []string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range names {
			if strings.EqualFold(existing, name) {
				return
			}
		}
		names = append(names, name)
	}
	add(main)
	add(parts[0])
	for _, part := range parts[1:] {
		for _, name := range featuredSeparator.Split(part, -1) {
			add(name)
		}
	}
	if len(names) == 0 {
		return []string{"Unknown artist"}
	}
	return names
}
