package types

import "testing"

func TestMatchLocaleTakesAnExactTag(t *testing.T) {
	for _, tag := range []string{"ja-JP", "pt-BR", "zh-TW", DefaultLocale} {
		if got := MatchLocale(tag); got != tag {
			t.Errorf("MatchLocale(%q) = %q, want it unchanged", tag, got)
		}
	}
}

func TestMatchLocaleIgnoresCaseAndSeparator(t *testing.T) {
	for _, tag := range []string{"JA-JP", "ja_JP", "Ja-jp"} {
		if got := MatchLocale(tag); got != "ja-JP" {
			t.Errorf("MatchLocale(%q) = %q, want ja-JP", tag, got)
		}
	}
}

// The Riot Client serves en-GB and valorant-api does not, which is the whole
// reason this falls back by language rather than demanding an exact tag.
func TestMatchLocaleFallsBackToTheOnlyTagForThatLanguage(t *testing.T) {
	if got := MatchLocale("en-GB"); got != DefaultLocale {
		t.Errorf("MatchLocale(\"en-GB\") = %q, want %q", got, DefaultLocale)
	}
	if got := MatchLocale("de-AT"); got != "de-DE" {
		t.Errorf("MatchLocale(\"de-AT\") = %q, want de-DE", got)
	}
}

// Spanish and Chinese have two tags each, so an unlisted regional variant
// has to land somewhere predictable rather than at random.
func TestMatchLocalePicksTheFirstTagForAnAmbiguousLanguage(t *testing.T) {
	cases := map[string]string{
		"es-AR": "es-ES",
		"zh-HK": "zh-CN",
	}

	for in, want := range cases {
		if got := MatchLocale(in); got != want {
			t.Errorf("MatchLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchLocaleFallsBackToEnglish(t *testing.T) {
	for _, tag := range []string{"", "   ", "xx-XX", "kl", "nonsense"} {
		if got := MatchLocale(tag); got != DefaultLocale {
			t.Errorf("MatchLocale(%q) = %q, want %q", tag, got, DefaultLocale)
		}
	}
}

// A bare language with no region is what a sloppy client build might send.
func TestMatchLocaleAcceptsABareLanguage(t *testing.T) {
	if got := MatchLocale("ja"); got != "ja-JP" {
		t.Errorf("MatchLocale(\"ja\") = %q, want ja-JP", got)
	}
}

func TestResolveLocalePrefersAnExplicitSetting(t *testing.T) {
	if got := ResolveLocale("ja-JP", "de-DE"); got != "ja-JP" {
		t.Errorf("ResolveLocale = %q, want the configured ja-JP", got)
	}
}

func TestResolveLocaleFollowsTheClientWhenSetToAuto(t *testing.T) {
	if got := ResolveLocale(LocaleAuto, "de-DE"); got != "de-DE" {
		t.Errorf("ResolveLocale = %q, want the client's de-DE", got)
	}
	if got := ResolveLocale(LocaleAuto, "en-GB"); got != DefaultLocale {
		t.Errorf("ResolveLocale = %q, want the matched %q", got, DefaultLocale)
	}
}

// Auto before the client has been read, and any value that survived a broken
// config, both have to land on English rather than on nothing.
func TestResolveLocaleFallsBackToEnglish(t *testing.T) {
	if got := ResolveLocale(LocaleAuto, ""); got != DefaultLocale {
		t.Errorf("ResolveLocale with no client locale = %q, want %q", got, DefaultLocale)
	}
	if got := ResolveLocale("", ""); got != DefaultLocale {
		t.Errorf("ResolveLocale with nothing set = %q, want %q", got, DefaultLocale)
	}
	if got := ResolveLocale("xx-XX", "de-DE"); got != DefaultLocale {
		t.Errorf("ResolveLocale with a bogus setting = %q, want %q", got, DefaultLocale)
	}
}
