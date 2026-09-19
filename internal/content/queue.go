package content

import (
	"strings"
	"unicode"
)

// queueNames maps Riot's queue strings to display names. valorant-api.com
// returns queueID: null on every game mode, so there is nothing to join on.
var queueNames = map[string]string{
	"competitive": "Competitive",
	"unrated":     "Unrated",
	"swiftplay":   "Swiftplay",
	"spikerush":   "Spike Rush",
	"deathmatch":  "Deathmatch",
	"ggteam":      "Escalation",
	"hurm":        "Team Deathmatch",
	"onefa":       "Replication",
	"newmap":      "New Map",
	"snowball":    "Snowball Fight",
	"premier":     "Premier",
}

// QueueName is the display name for a queue ID. One the table does not know
// title-cases from the raw string, never "Unknown"; a blank ID stays blank.
func QueueName(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	if name, ok := queueNames[strings.ToLower(trimmed)]; ok {
		return name
	}
	return titleCase(trimmed)
}

// titleCase splits on anything that is not a letter or digit and upper-cases
// each word's first rune, leaving the rest alone so camelCase survives.
func titleCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	for i, w := range words {
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
