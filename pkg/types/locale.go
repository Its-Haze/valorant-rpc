package types

import (
	"slices"
	"strings"
)

// DefaultLocale is the language names fall back to. valorant-api always
// carries it, so it is the one tag guaranteed to be present in every payload.
const DefaultLocale = "en-US"

// Locale is one of the languages valorant-api serves. Name is the language's
// own name, which is what a language picker should show.
type Locale struct {
	Tag  string
	Name string
}

// locales is every key valorant-api puts in a displayName map under
// ?language=all. English leads; the rest sort by tag so the list is stable.
var locales = []Locale{
	{Tag: DefaultLocale, Name: "English"},
	{Tag: "ar-AE", Name: "العربية"},
	{Tag: "de-DE", Name: "Deutsch"},
	{Tag: "es-ES", Name: "Español (España)"},
	{Tag: "es-MX", Name: "Español (México)"},
	{Tag: "fr-FR", Name: "Français"},
	{Tag: "id-ID", Name: "Bahasa Indonesia"},
	{Tag: "it-IT", Name: "Italiano"},
	{Tag: "ja-JP", Name: "日本語"},
	{Tag: "ko-KR", Name: "한국어"},
	{Tag: "pl-PL", Name: "Polski"},
	{Tag: "pt-BR", Name: "Português (Brasil)"},
	{Tag: "ru-RU", Name: "Русский"},
	{Tag: "th-TH", Name: "ไทย"},
	{Tag: "tr-TR", Name: "Türkçe"},
	{Tag: "vi-VN", Name: "Tiếng Việt"},
	{Tag: "zh-CN", Name: "简体中文"},
	{Tag: "zh-TW", Name: "繁體中文"},
}

// Locales is every language the catalogue can render, for the settings
// dropdown. The caller gets a copy it is free to reorder.
func Locales() []Locale { return slices.Clone(locales) }

// ValidLocale reports a tag the catalogue knows. Tags are compared exactly,
// because they are picked from a list rather than typed.
func ValidLocale(tag string) bool {
	return slices.ContainsFunc(locales, func(l Locale) bool { return l.Tag == tag })
}

// LocaleAuto is the display.locale setting that follows the Riot Client's own
// language instead of pinning one. It is the default.
const LocaleAuto = "auto"

// MatchLocale maps the Riot Client's locale onto one the catalogue serves.
// The client offers languages valorant-api does not, en-GB being the one that
// actually ships, so an unlisted tag falls back by language before English.
func MatchLocale(clientLocale string) string {
	tag := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(clientLocale, "_", "-")))
	if tag == "" {
		return DefaultLocale
	}

	for _, l := range locales {
		if strings.EqualFold(l.Tag, tag) {
			return l.Tag
		}
	}

	// List order decides for a language with several tags, so es-AR lands on
	// es-ES every time rather than on whichever entry was seen first.
	language, _, _ := strings.Cut(tag, "-")
	for _, l := range locales {
		if prefix, _, _ := strings.Cut(strings.ToLower(l.Tag), "-"); prefix == language {
			return l.Tag
		}
	}

	return DefaultLocale
}

// ResolveLocale turns the configured setting and the Riot Client's language
// into the tag a presence renders in. Anything unusable lands on English.
func ResolveLocale(configured, clientLocale string) string {
	if configured == LocaleAuto {
		return MatchLocale(clientLocale)
	}
	if ValidLocale(configured) {
		return configured
	}
	return DefaultLocale
}
