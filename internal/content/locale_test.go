package content

import (
	"encoding/json"
	"sort"
	"testing"
)

// The locale list is hand-maintained, so it has to be checked against what
// the payloads actually carry rather than trusted.
func TestLocalesMatchTheFixturePayloads(t *testing.T) {
	var payload struct {
		Data []struct {
			DisplayName map[string]string `json:"displayName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(readFixture(t, "agents.json"), &payload); err != nil {
		t.Fatalf("decoding the agents fixture: %v", err)
	}
	if len(payload.Data) == 0 {
		t.Fatal("the agents fixture is empty")
	}

	// The fixtures are trimmed to a few locales, so every tag they carry has
	// to be known, not the other way round.
	for tag := range payload.Data[0].DisplayName {
		if !ValidLocale(tag) {
			t.Errorf("the fixture carries %q, which Locales() does not list", tag)
		}
	}
}

func TestLocalesAreUniqueAndNamed(t *testing.T) {
	seen := make(map[string]bool)

	for _, l := range Locales() {
		if l.Tag == "" || l.Name == "" {
			t.Errorf("incomplete locale: %+v", l)
		}
		if seen[l.Tag] {
			t.Errorf("%q is listed twice", l.Tag)
		}
		seen[l.Tag] = true
	}

	if len(seen) != 18 {
		t.Errorf("listed %d locales, want the 18 valorant-api serves", len(seen))
	}
}

// English leads because it is the default and the fallback; the rest are
// ordered so the settings dropdown does not reshuffle between builds.
func TestLocalesLeadWithEnglishAndAreOtherwiseSorted(t *testing.T) {
	locales := Locales()

	if locales[0].Tag != DefaultLocale {
		t.Fatalf("first locale is %q, want %q", locales[0].Tag, DefaultLocale)
	}

	rest := make([]string, 0, len(locales)-1)
	for _, l := range locales[1:] {
		rest = append(rest, l.Tag)
	}
	if !sort.StringsAreSorted(rest) {
		t.Errorf("locales after English are not sorted: %v", rest)
	}
}

func TestValidLocale(t *testing.T) {
	for _, tag := range []string{DefaultLocale, "ja-JP", "pt-BR", "zh-TW"} {
		if !ValidLocale(tag) {
			t.Errorf("ValidLocale(%q) = false, want true", tag)
		}
	}
	for _, tag := range []string{"", "en", "en-GB", "xx-XX", "EN-US"} {
		if ValidLocale(tag) {
			t.Errorf("ValidLocale(%q) = true, want false", tag)
		}
	}
}

// Mutating the returned slice must not corrupt the package's own list.
func TestLocalesReturnsACopy(t *testing.T) {
	Locales()[0] = Locale{Tag: "xx-XX", Name: "clobbered"}

	if Locales()[0].Tag != DefaultLocale {
		t.Error("Locales() hands out its backing array")
	}
}
