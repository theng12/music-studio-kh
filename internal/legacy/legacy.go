// Package legacy moves data an earlier release kept under an old folder name.
package legacy

import (
	"os"
	"path/filepath"
)

// Move renames oldDir to newDir once: only when newDir is absent and oldDir
// exists. A failed move leaves both paths untouched, so the caller can keep
// using oldDir rather than starting with no data.
func Move(newDir, oldDir string) (bool, error) {
	if _, err := os.Stat(newDir); err == nil {
		return false, nil
	}
	if _, err := os.Stat(oldDir); err != nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0700); err != nil {
		return false, err
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return false, err
	}
	return true, nil
}
