package riotclient

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestRegionLocaleReadsTheClientsOwnLanguage(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	got, err := client.RegionLocale(context.Background())
	if err != nil {
		t.Fatalf("RegionLocale: %v", err)
	}
	if got.Locale != "en-GB" {
		t.Errorf("Locale = %q, want en-GB", got.Locale)
	}
}

func TestRegionLocaleCarriesRegionAndWebRegion(t *testing.T) {
	riot := newFakeRiot(t)
	riot.route(healthEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"locale":"ja-JP","region":"AP","webRegion":"ap"}`)
	})
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	got, err := client.RegionLocale(context.Background())
	if err != nil {
		t.Fatalf("RegionLocale: %v", err)
	}
	if got.Locale != "ja-JP" || got.Region != "AP" || got.WebRegion != "ap" {
		t.Errorf("got %+v, want the whole payload", got)
	}
}

func TestRegionLocaleNeedsAConnection(t *testing.T) {
	client := New(Options{Logger: discardLogger()})

	if _, err := client.RegionLocale(context.Background()); err == nil {
		t.Error("RegionLocale succeeded without a connection")
	}
}

func TestRegionLocaleRejectsANonOKStatus(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Routed only after Connect, so the health check still passes and the
	// failure is the locale read's alone.
	riot.route(healthEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := client.RegionLocale(context.Background()); err == nil {
		t.Error("a 500 reported success")
	}
}

func TestRegionLocaleRejectsAMalformedBody(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	riot.route(healthEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"locale":`)
	})

	if _, err := client.RegionLocale(context.Background()); err == nil {
		t.Error("a truncated body reported success")
	}
}
