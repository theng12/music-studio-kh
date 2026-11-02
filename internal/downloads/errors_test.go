package downloads

import (
	"errors"
	"strings"
	"testing"
)

func TestJobErrorsExplainKnownFailuresWithoutLeakingDetails(t *testing.T) {
	for _, input := range []string{
		"failed writing music folder /Users/private/audio.mp3",
		"request failed with Authorization: Bearer fixture-value",
		"request failed: PASSWORD=fixture-value",
		"Cookie: fixture-value",
		`failed writing C:\private\audio.mp3`,
	} {
		got := sanitizeJobError(errors.New(input))
		if got == "" || strings.Contains(got, "private") || strings.Contains(got, "fixture-value") {
			t.Fatalf("unsafe or missing error for diagnostic %q: %q", input, got)
		}
	}
	for _, test := range []struct{ input, want string }{
		{"Postprocessing: Encoder not found", "bundled encoder"},
		{"No space left on device: /private/audio.mp3", "free disk space"},
		{"This video is unavailable", "Check that the link opens on YouTube"},
		{"Private video", "video is private"},
	} {
		got := sanitizeJobError(errors.New(test.input))
		if !strings.Contains(got, test.want) || strings.Contains(got, "/private") {
			t.Fatalf("error %q: got %q, want safe explanation containing %q", test.input, got, test.want)
		}
	}
}
