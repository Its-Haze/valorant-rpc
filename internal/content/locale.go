package content

import "slices"

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
