package content

import (
	"encoding/json"
	"testing"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

// The locale list in pkg/types is hand-maintained, so it has to be checked
// against what the payloads actually carry rather than trusted.
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
		if !types.ValidLocale(tag) {
			t.Errorf("the fixture carries %q, which types.Locales() does not list", tag)
		}
	}
}
