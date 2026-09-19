package discord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The app's own icon is hotlinked out of this repository's assets/ folder, so
// deleting or renaming it silently breaks every user's presence.
func TestLogoURLsPointAtCommittedAssets(t *testing.T) {
	const prefix = "https://github.com/Its-Haze/valorant-rpc/blob/main/"

	for name, url := range map[string]string{
		"valorantLogoURL":     valorantLogoURL,
		"valorantLogoIdleURL": valorantLogoIdleURL,
	} {
		rel, ok := strings.CutPrefix(url, prefix)
		if !ok {
			t.Errorf("%s = %q, want it to start with %q; update this test if the assets moved host", name, url, prefix)
			continue
		}
		rel = strings.TrimSuffix(rel, "?raw=true")
		path := filepath.Join("..", "..", filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s points at %q, which is not committed: %v", name, rel, err)
		}
	}
}
