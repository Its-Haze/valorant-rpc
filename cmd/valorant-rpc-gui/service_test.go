package main

import (
	"testing"

	"github.com/its-haze/valorant-rpc/internal/app"
	"github.com/its-haze/valorant-rpc/internal/config"
)

func TestGUIService_SettingsRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	store := config.NewStore(config.DefaultConfig())
	svc := newGUIService(app.New(store, &fakePause{}))

	cfg := svc.GetSettings()
	cfg.Presence.ShowInClient = !cfg.Presence.ShowInClient
	cfg.Advanced.UpdateInterval = 2222
	want := cfg.Presence.ShowInClient

	if err := svc.ApplySettings(cfg); err != nil {
		t.Fatalf("ApplySettings: %v", err)
	}

	got := svc.GetSettings()
	if got.Presence.ShowInClient != want || got.Advanced.UpdateInterval != 2222 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestGUIService_ApplyRejectsInvalid(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	svc := newGUIService(app.New(config.NewStore(config.DefaultConfig()), &fakePause{}))

	cfg := svc.GetSettings()
	cfg.DiscordAppID = ""

	if err := svc.ApplySettings(cfg); err == nil {
		t.Fatal("ApplySettings accepted an empty DiscordAppID")
	}
	if svc.GetSettings().DiscordAppID == "" {
		t.Fatal("rejected ApplySettings still mutated live settings")
	}
}
