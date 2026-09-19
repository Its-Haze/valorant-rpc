package config

import (
	"encoding/json"
	"testing"

	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/internal/presence/template"
)

func TestDefaultConfig_ShipsEveryPresenceTemplate(t *testing.T) {
	got := DefaultConfig().Presence.Templates
	for _, ctx := range template.Contexts() {
		pair, ok := got[string(ctx)]
		if !ok {
			t.Fatalf("no default template for context %q", ctx)
		}
		wantD, wantS := template.Default(ctx)
		if pair.Details != wantD || pair.State != wantS {
			t.Errorf("context %q = %+v, want {%q %q}", ctx, pair, wantD, wantS)
		}
	}
}

func TestClamp_BackfillsMissingTemplatesButKeepsUserEdits(t *testing.T) {
	const edited = "edited-by-hand"

	c := DefaultConfig()
	c.Presence.Templates = map[string]TemplatePair{
		edited: {Details: "custom {mode}", State: "custom"},
	}
	c.clamp()

	if c.Presence.Templates[edited].Details != "custom {mode}" {
		t.Error("clamp overwrote a user-set template")
	}
	// Vacuous until the presence builders register their contexts, and the
	// assertion that matters from then on.
	for _, ctx := range template.Contexts() {
		if _, ok := c.Presence.Templates[string(ctx)]; !ok {
			t.Errorf("clamp did not backfill the missing %q template", ctx)
		}
	}
}

func TestValidate_ReportsEveryProblemAtOnce(t *testing.T) {
	c := DefaultConfig()
	c.DiscordAppID = ""
	c.Theme = "neon"
	c.Behavior.CloseAction = "explode"
	c.Advanced.UpdateInterval = 10

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted an invalid config")
	}
	for _, want := range []string{"discord_app_id", "theme", "close_action", "update_interval"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q missing mention of %q", err, want)
		}
	}
}

func TestValidate_DoesNotMutate(t *testing.T) {
	c := DefaultConfig()
	c.DiscordAppID = ""
	c.Advanced.UpdateInterval = 10

	before, _ := json.Marshal(c)
	_ = c.Validate()
	after, _ := json.Marshal(c)

	if string(before) != string(after) {
		t.Fatalf("Validate mutated the config:\n%s\n%s", before, after)
	}
}

func TestValidate_AcceptsDefault(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig failed validation: %v", err)
	}
}

func TestClamp_RepairsOutOfBounds(t *testing.T) {
	c := &Config{
		Theme:    "bogus",
		Advanced: AdvancedConfig{UpdateInterval: 50},
	}
	c.clamp()

	if c.DiscordAppID == "" {
		t.Error("clamp left DiscordAppID empty")
	}
	if c.Theme != ThemeSystem {
		t.Errorf("Theme = %q, want %q", c.Theme, ThemeSystem)
	}
	// A file written before close_action existed has it empty; asking is the
	// only safe repair, since quitting silently would surprise the user.
	if c.Behavior.CloseAction != CloseAsk {
		t.Errorf("CloseAction = %q, want %q", c.Behavior.CloseAction, CloseAsk)
	}
	if c.Advanced.UpdateInterval != DefaultConfig().Advanced.UpdateInterval {
		t.Errorf("UpdateInterval = %d, want default", c.Advanced.UpdateInterval)
	}
	if c.Presence.Templates == nil {
		t.Error("clamp left nested maps nil")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("config still invalid after clamp: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestValidate_RejectsAnUnknownLocale(t *testing.T) {
	c := DefaultConfig()
	c.Display.Locale = "en-GB"

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a locale valorant-api does not serve")
	}
	if !contains(err.Error(), "locale") {
		t.Errorf("error %q does not mention the locale", err)
	}
}

func TestValidate_AcceptsEveryLocaleTheCatalogueServes(t *testing.T) {
	for _, l := range content.Locales() {
		c := DefaultConfig()
		c.Display.Locale = l.Tag

		if err := c.Validate(); err != nil {
			t.Errorf("Validate rejected %q: %v", l.Tag, err)
		}
	}
}

func TestClamp_RepairsAnUnknownLocale(t *testing.T) {
	c := &Config{Display: DisplayConfig{Locale: "kl-KL"}}
	c.clamp()

	if c.Display.Locale != content.DefaultLocale {
		t.Errorf("Locale = %q, want %q", c.Display.Locale, content.DefaultLocale)
	}
}

// A config file written before the field existed has it empty, and must boot
// on English rather than blanking every name in the presence.
func TestClamp_FillsAnEmptyLocale(t *testing.T) {
	c := &Config{}
	c.clamp()

	if c.Display.Locale != content.DefaultLocale {
		t.Errorf("Locale = %q, want %q", c.Display.Locale, content.DefaultLocale)
	}
}
