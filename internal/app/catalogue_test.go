package app

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/content"
)

// fixtureDoer serves the content package's committed payloads, so the preview
// tests resolve art against the same data a real refresh would load.
type fixtureDoer struct{ t *testing.T }

func (d fixtureDoer) Do(req *http.Request) (*http.Response, error) {
	names := map[string]string{
		"/agents": "agents.json", "/maps": "maps.json",
		"/competitivetiers": "competitivetiers.json", "/gamemodes": "gamemodes.json",
		"/playercards": "playercards.json",
	}

	name, ok := names[req.URL.Path[strings.LastIndex(req.URL.Path, "/"):]]
	if !ok {
		d.t.Fatalf("unexpected catalogue request %s", req.URL.Path)
	}
	blob, err := os.ReadFile(filepath.Join("..", "content", "testdata", name))
	if err != nil {
		d.t.Fatalf("reading %s: %v", name, err)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(blob))}, nil
}

func testCatalogue(t *testing.T) *content.Catalogue {
	t.Helper()

	cache := content.New(content.Options{Doer: fixtureDoer{t}, Logger: zerolog.Nop()})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("loading the fixture catalogue: %v", err)
	}
	return cache.Snapshot()
}

// fakeCatalogue hands a fixed snapshot to WithCatalogue, standing in for the
// *content.Cache the GUI wires in.
type fakeCatalogue struct{ cat *content.Catalogue }

func (f fakeCatalogue) Snapshot() *content.Catalogue { return f.cat }
