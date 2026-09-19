package discord

import (
	"image/png"
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
		"valorantLogoURL":           valorantLogoURL,
		"valorantLogoBorderlessURL": valorantLogoBorderlessURL,
		"valorantLogoIdleURL":       valorantLogoIdleURL,
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

// Discord draws presence art on whatever background the viewer's theme gives
// it, so a transparent edge reads as a notch cut out of the icon.
func TestCommittedLogosAreFullyOpaque(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "assets", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no committed assets found: %v", err)
	}

	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("opening %s: %v", path, err)
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("decoding %s: %v", path, err)
			continue
		}

		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
					t.Errorf("%s is transparent at (%d,%d); the art has to fill the whole canvas", filepath.Base(path), x, y)
					y, x = b.Max.Y, b.Max.X
				}
			}
		}
	}
}
