package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	jettUUID   = "add6443a-41bd-e414-f6ad-e58d267f4e95"
	ascentURL  = "/Game/Maps/Ascent/Ascent"
	rangeURL   = "/Game/Maps/Poveglia/Range"
	rangeV2URL = "/Game/Maps/PovegliaV2/RangeV2"
	bombMode   = "ShooterGame/Content/GameModes/Bomb/BombGameMode_PrimaryAsset"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	blob, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return blob
}

// fixtureCatalogue parses the four committed payloads exactly as a real
// refresh would.
func fixtureCatalogue(t *testing.T) *Catalogue {
	t.Helper()

	cat, err := parseCatalogue(
		readFixture(t, "agents.json"),
		readFixture(t, "maps.json"),
		readFixture(t, "competitivetiers.json"),
		readFixture(t, "gamemodes.json"),
	)
	if err != nil {
		t.Fatalf("parsing the fixture catalogue: %v", err)
	}
	return cat
}

func TestAgentLookupJoinsOnUUID(t *testing.T) {
	cat := fixtureCatalogue(t)

	agent, ok := cat.Agent(jettUUID, DefaultLocale)
	if !ok {
		t.Fatalf("Jett did not resolve")
	}
	if agent.Name != "Jett" {
		t.Errorf("Name = %q, want Jett", agent.Name)
	}
	if !strings.HasPrefix(agent.Icon, "https://media.valorant-api.com/agents/") {
		t.Errorf("Icon = %q, want a media.valorant-api.com URL", agent.Icon)
	}
	if agent.IconSmall == "" {
		t.Error("IconSmall is empty")
	}
	if len(agent.GradientColors) == 0 {
		t.Error("GradientColors is empty; the gradient is carried through for later use")
	}
}

// glz returns agent UUIDs uppercase, valorant-api returns them lowercase.
func TestAgentLookupIsCaseInsensitiveInBothDirections(t *testing.T) {
	cat := fixtureCatalogue(t)

	upper, ok := cat.Agent(strings.ToUpper(jettUUID), DefaultLocale)
	if !ok {
		t.Fatalf("an uppercase UUID did not resolve")
	}
	lower, ok := cat.Agent(strings.ToLower(jettUUID), DefaultLocale)
	if !ok {
		t.Fatalf("a lowercase UUID did not resolve")
	}
	if upper.UUID != lower.UUID {
		t.Errorf("case changed the result: %q vs %q", upper.UUID, lower.UUID)
	}
}

func TestAgentNameFollowsTheLocale(t *testing.T) {
	cat := fixtureCatalogue(t)

	ja, ok := cat.Agent(jettUUID, "ja-JP")
	if !ok {
		t.Fatalf("Jett did not resolve")
	}
	if ja.Name == "" || ja.Name == "Jett" {
		t.Errorf("ja-JP name = %q, want the localized string", ja.Name)
	}
}

func TestAgentNameFallsBackToEnglishForAnUnknownLocale(t *testing.T) {
	cat := fixtureCatalogue(t)

	agent, ok := cat.Agent(jettUUID, "xx-XX")
	if !ok {
		t.Fatalf("Jett did not resolve")
	}
	if agent.Name != "Jett" {
		t.Errorf("Name = %q, want the en-US fallback Jett", agent.Name)
	}
}

func TestUnknownAgentDoesNotResolve(t *testing.T) {
	cat := fixtureCatalogue(t)

	if _, ok := cat.Agent("00000000-0000-0000-0000-000000000000", DefaultLocale); ok {
		t.Error("an unknown UUID resolved")
	}
}

func TestMapLookupJoinsOnMapURL(t *testing.T) {
	cat := fixtureCatalogue(t)

	m, ok := cat.Map(ascentURL, DefaultLocale)
	if !ok {
		t.Fatalf("Ascent did not resolve")
	}
	if m.Name != "Ascent" {
		t.Errorf("Name = %q, want Ascent", m.Name)
	}
	if m.ListViewIcon == "" || m.Splash == "" {
		t.Errorf("missing art: listViewIcon=%q splash=%q", m.ListViewIcon, m.Splash)
	}
	if m.URL != ascentURL {
		t.Errorf("URL = %q, want %q", m.URL, ascentURL)
	}
}

func TestMapLookupIsCaseInsensitiveInBothDirections(t *testing.T) {
	cat := fixtureCatalogue(t)

	if _, ok := cat.Map(strings.ToUpper(ascentURL), DefaultLocale); !ok {
		t.Error("an uppercase mapUrl did not resolve")
	}
	if _, ok := cat.Map(strings.ToLower(ascentURL), DefaultLocale); !ok {
		t.Error("a lowercase mapUrl did not resolve")
	}
}

// Two map entries are called "The Range". Neither is hardcoded and both have
// to resolve, because which one the client reports depends on the build.
func TestBothRangeMapsResolve(t *testing.T) {
	cat := fixtureCatalogue(t)

	for _, url := range []string{rangeURL, rangeV2URL} {
		m, ok := cat.Map(url, DefaultLocale)
		if !ok {
			t.Fatalf("%s did not resolve", url)
		}
		if m.Name != "The Range" {
			t.Errorf("%s resolved to %q, want The Range", url, m.Name)
		}
	}
}

// Five Skirmish variants ship alongside the competitive maps. They resolve
// like anything else; nothing filters the list down to ranked maps.
func TestSkirmishMapsResolve(t *testing.T) {
	cat := fixtureCatalogue(t)

	m, ok := cat.Map("/Game/Maps/Duel/Duel_1/Skirmish_A", DefaultLocale)
	if !ok {
		t.Fatalf("Skirmish A did not resolve")
	}
	if m.Name != "Skirmish A" {
		t.Errorf("Name = %q, want Skirmish A", m.Name)
	}
}

func TestTierTableComesFromTheLastEntry(t *testing.T) {
	cat := fixtureCatalogue(t)

	radiant, ok := cat.Tier(RadiantTier, DefaultLocale)
	if !ok {
		t.Fatalf("tier %d did not resolve", RadiantTier)
	}
	if !strings.EqualFold(radiant.Name, "RADIANT") {
		t.Errorf("tier %d name = %q, want RADIANT", RadiantTier, radiant.Name)
	}
	if radiant.LargeIcon == "" {
		t.Error("Radiant has no largeIcon")
	}
}

func TestTierIndexingAtTheBoundaries(t *testing.T) {
	cat := fixtureCatalogue(t)

	unranked, ok := cat.Tier(0, DefaultLocale)
	if !ok {
		t.Fatalf("tier 0 did not resolve")
	}
	if !strings.EqualFold(unranked.Name, "UNRANKED") {
		t.Errorf("tier 0 name = %q, want UNRANKED", unranked.Name)
	}

	// Tiers 1 and 2 are "Unused" placeholders with an INVALID division. No
	// player holds them and their names must never reach a presence.
	for _, tier := range []int{1, 2} {
		if got, ok := cat.Tier(tier, DefaultLocale); ok {
			t.Errorf("tier %d resolved to %q, want no result", tier, got.Name)
		}
	}

	if _, ok := cat.Tier(RadiantTier+1, DefaultLocale); ok {
		t.Errorf("tier %d resolved, want no result", RadiantTier+1)
	}
	if _, ok := cat.Tier(-1, DefaultLocale); ok {
		t.Error("tier -1 resolved, want no result")
	}
}

func TestIronOneIsTheFirstRealTier(t *testing.T) {
	cat := fixtureCatalogue(t)

	iron, ok := cat.Tier(3, DefaultLocale)
	if !ok {
		t.Fatalf("tier 3 did not resolve")
	}
	if !strings.EqualFold(iron.Name, "IRON 1") {
		t.Errorf("tier 3 name = %q, want IRON 1", iron.Name)
	}
	if iron.LargeIcon == "" {
		t.Error("tier 3 has no largeIcon")
	}
}

func TestGameModeLookupJoinsOnAssetPath(t *testing.T) {
	cat := fixtureCatalogue(t)

	mode, ok := cat.GameMode(strings.ToUpper(bombMode), DefaultLocale)
	if !ok {
		t.Fatalf("the Bomb game mode did not resolve")
	}
	if mode.Name != "Standard" {
		t.Errorf("Name = %q, want Standard", mode.Name)
	}
}

func TestEmptyCatalogueResolvesNothing(t *testing.T) {
	var cat Catalogue

	if !cat.Empty() {
		t.Error("a zero Catalogue is not reporting empty")
	}
	if _, ok := cat.Agent(jettUUID, DefaultLocale); ok {
		t.Error("an empty catalogue resolved an agent")
	}
	if _, ok := cat.Map(ascentURL, DefaultLocale); ok {
		t.Error("an empty catalogue resolved a map")
	}
	if _, ok := cat.Tier(RadiantTier, DefaultLocale); ok {
		t.Error("an empty catalogue resolved a tier")
	}
	if _, ok := cat.GameMode(bombMode, DefaultLocale); ok {
		t.Error("an empty catalogue resolved a game mode")
	}
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	_, err := parseCatalogue([]byte("{"), nil, nil, nil)
	if err == nil {
		t.Error("parsing a truncated payload returned no error")
	}
}
