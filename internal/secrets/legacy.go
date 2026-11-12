package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// readLegacyKey is the one-time bridge for profiles created before 0.3.5, when
// the credential key lived in the operating system's credential store. It is
// called only from migrateLegacy, before secrets.json is marked migrated;
// every later run reads the local file and never reaches this function.
func readLegacyKey(root, id string) (string, bool) {
	absolute, _ := filepath.Abs(root)
	h := sha256.Sum256([]byte(absolute))
	prefix := hex.EncodeToString(h[:8]) + ":"
	value, e := keyring.Get("MusicStudioKH", prefix+id)
	return value, e == nil
}
