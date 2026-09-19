package content

import "testing"

func TestQueueNameUsesTheHardcodedTable(t *testing.T) {
	cases := map[string]string{
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
		"fortcollins": "Retake",
		"skirmish2v2": "Skirmish",
	}

	for id, want := range cases {
		if got := QueueName(id); got != want {
			t.Errorf("QueueName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestQueueNameMatchesCaseInsensitively(t *testing.T) {
	if got := QueueName("COMPETITIVE"); got != "Competitive" {
		t.Errorf("QueueName(\"COMPETITIVE\") = %q, want %q", got, "Competitive")
	}
}

func TestQueueNameTitleCasesAnUnknownID(t *testing.T) {
	cases := map[string]string{
		"aros":            "Aros",
		"spike_rush_2":    "Spike Rush 2",
		"team-deathmatch": "Team Deathmatch",
		"someNewMode":     "SomeNewMode",
	}

	for id, want := range cases {
		if got := QueueName(id); got != want {
			t.Errorf("QueueName(%q) = %q, want %q", id, got, want)
		}
	}
}

// An empty queue ID is a custom game, which the custom-game builder already
// names. It must never render as "Unknown".
func TestQueueNameOfAnEmptyIDIsEmpty(t *testing.T) {
	if got := QueueName("   "); got != "" {
		t.Errorf("QueueName of blank = %q, want empty", got)
	}
}
