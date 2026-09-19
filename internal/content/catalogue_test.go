package content

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

const (
	jettUUID   = "add6443a-41bd-e414-f6ad-e58d267f4e95"
	ascentURL  = "/Game/Maps/Ascent/Ascent"
	rangeURL   = "/Game/Maps/Poveglia/Range"
	rangeV2URL = "/Game/Maps/PovegliaV2/RangeV2"
	bombMode   = "ShooterGame/Content/GameModes/Bomb/BombGameMode_PrimaryAsset"

	sampleCardUUID = "1711d20d-4b1c-c64a-14be-d4ae58a457c6"
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
		readFixture(t, "playercards.json"),
	)
	if err != nil {
		t.Fatalf("parsing the fixture catalogue: %v", err)
	}
	return cat
}

func TestAgentLookupJoinsOnUUID(t *testing.T) {
	cat := fixtureCatalogue(t)

	agent, ok := cat.Agent(jettUUID, types.DefaultLocale)
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

	upper, ok := cat.Agent(strings.ToUpper(jettUUID), types.DefaultLocale)
	if !ok {
		t.Fatalf("an uppercase UUID did not resolve")
	}
	lower, ok := cat.Agent(strings.ToLower(jettUUID), types.DefaultLocale)
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

	if _, ok := cat.Agent("00000000-0000-0000-0000-000000000000", types.DefaultLocale); ok {
		t.Error("an unknown UUID resolved")
	}
}

func TestMapLookupJoinsOnMapURL(t *testing.T) {
	cat := fixtureCatalogue(t)

	m, ok := cat.Map(ascentURL, types.DefaultLocale)
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

	if _, ok := cat.Map(strings.ToUpper(ascentURL), types.DefaultLocale); !ok {
		t.Error("an uppercase mapUrl did not resolve")
	}
	if _, ok := cat.Map(strings.ToLower(ascentURL), types.DefaultLocale); !ok {
		t.Error("a lowercase mapUrl did not resolve")
	}
}

// Two map entries are called "The Range". Neither is hardcoded and both have
// to resolve, because which one the client reports depends on the build.
func TestBothRangeMapsResolve(t *testing.T) {
	cat := fixtureCatalogue(t)

	for _, url := range []string{rangeURL, rangeV2URL} {
		m, ok := cat.Map(url, types.DefaultLocale)
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

	m, ok := cat.Map("/Game/Maps/Duel/Duel_1/Skirmish_A", types.DefaultLocale)
	if !ok {
		t.Fatalf("Skirmish A did not resolve")
	}
	if m.Name != "Skirmish A" {
		t.Errorf("Name = %q, want Skirmish A", m.Name)
	}
}

func TestTierTableComesFromTheLastEntry(t *testing.T) {
	cat := fixtureCatalogue(t)

	radiant, ok := cat.Tier(RadiantTier, types.DefaultLocale)
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

	unranked, ok := cat.Tier(0, types.DefaultLocale)
	if !ok {
		t.Fatalf("tier 0 did not resolve")
	}
	if !strings.EqualFold(unranked.Name, "UNRANKED") {
		t.Errorf("tier 0 name = %q, want UNRANKED", unranked.Name)
	}

	// Tiers 1 and 2 are "Unused" placeholders with an INVALID division. No
	// player holds them and their names must never reach a presence.
	for _, tier := range []int{1, 2} {
		if got, ok := cat.Tier(tier, types.DefaultLocale); ok {
			t.Errorf("tier %d resolved to %q, want no result", tier, got.Name)
		}
	}

	if _, ok := cat.Tier(RadiantTier+1, types.DefaultLocale); ok {
		t.Errorf("tier %d resolved, want no result", RadiantTier+1)
	}
	if _, ok := cat.Tier(-1, types.DefaultLocale); ok {
		t.Error("tier -1 resolved, want no result")
	}
}

func TestIronOneIsTheFirstRealTier(t *testing.T) {
	cat := fixtureCatalogue(t)

	iron, ok := cat.Tier(3, types.DefaultLocale)
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

	mode, ok := cat.GameMode(strings.ToUpper(bombMode), types.DefaultLocale)
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
	if _, ok := cat.Agent(jettUUID, types.DefaultLocale); ok {
		t.Error("an empty catalogue resolved an agent")
	}
	if _, ok := cat.Map(ascentURL, types.DefaultLocale); ok {
		t.Error("an empty catalogue resolved a map")
	}
	if _, ok := cat.Tier(RadiantTier, types.DefaultLocale); ok {
		t.Error("an empty catalogue resolved a tier")
	}
	if _, ok := cat.GameMode(bombMode, types.DefaultLocale); ok {
		t.Error("an empty catalogue resolved a game mode")
	}
	if _, ok := cat.PlayerCard(sampleCardUUID); ok {
		t.Error("an empty catalogue resolved a player card")
	}
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	_, err := parseCatalogue([]byte("{"), nil, nil, nil, nil)
	if err == nil {
		t.Error("parsing a truncated payload returned no error")
	}
}

func TestPlayerCardLookupJoinsOnUUID(t *testing.T) {
	cat := fixtureCatalogue(t)

	card, ok := cat.PlayerCard(sampleCardUUID)
	if !ok {
		t.Fatalf("the sample card did not resolve")
	}
	if !strings.HasPrefix(card.Icon, "https://media.valorant-api.com/playercards/") {
		t.Errorf("Icon = %q, want a playercards media URL", card.Icon)
	}
	if card.WideArt == "" || card.LargeArt == "" {
		t.Errorf("missing art: wide=%q large=%q", card.WideArt, card.LargeArt)
	}
}

// The presence blob's casing is not guaranteed, same as every other UUID.
func TestPlayerCardLookupIsCaseInsensitive(t *testing.T) {
	cat := fixtureCatalogue(t)

	if _, ok := cat.PlayerCard(strings.ToUpper(sampleCardUUID)); !ok {
		t.Error("an uppercase card UUID did not resolve")
	}
}

func TestUnknownPlayerCardDoesNotResolve(t *testing.T) {
	cat := fixtureCatalogue(t)

	if _, ok := cat.PlayerCard("00000000-0000-0000-0000-000000000000"); ok {
		t.Error("an unknown card UUID resolved")
	}
	if _, ok := cat.PlayerCard(""); ok {
		t.Error("an empty card UUID resolved")
	}
}

func TestEnumeratorsCoverTheWholeTable(t *testing.T) {
	cat := fixtureCatalogue(t)

	if got := len(cat.Agents(types.DefaultLocale)); got != len(cat.agents) {
		t.Errorf("Agents() returned %d, want %d", got, len(cat.agents))
	}
	if got := len(cat.Maps(types.DefaultLocale)); got != len(cat.maps) {
		t.Errorf("Maps() returned %d, want %d", got, len(cat.maps))
	}
	if got := len(cat.Tiers(types.DefaultLocale)); got != len(cat.tiers) {
		t.Errorf("Tiers() returned %d, want %d", got, len(cat.tiers))
	}
	if got := len(cat.PlayerCards()); got != len(cat.cards) {
		t.Errorf("PlayerCards() returned %d, want %d", got, len(cat.cards))
	}
}

// The asset liveness test samples these lists, so the order has to be the
// same on every run rather than Go's randomized map order.
func TestEnumeratorsAreDeterministic(t *testing.T) {
	cat := fixtureCatalogue(t)

	agentOrder := func() []string {
		var out []string
		for _, a := range cat.Agents(types.DefaultLocale) {
			out = append(out, a.UUID)
		}
		return out
	}
	mapOrder := func() []string {
		var out []string
		for _, m := range cat.Maps(types.DefaultLocale) {
			out = append(out, m.URL)
		}
		return out
	}

	agents, worlds := agentOrder(), mapOrder()
	for range 5 {
		if !slices.Equal(agentOrder(), agents) {
			t.Fatal("Agents() order changed between calls")
		}
		if !slices.Equal(mapOrder(), worlds) {
			t.Fatal("Maps() order changed between calls")
		}
	}
}

func TestTiersEnumerateInLadderOrderWithoutTheUnusedRows(t *testing.T) {
	tiers := fixtureCatalogue(t).Tiers(types.DefaultLocale)

	if len(tiers) == 0 {
		t.Fatal("no tiers enumerated")
	}
	if tiers[0].Tier != 0 {
		t.Errorf("first tier is %d, want 0 (Unranked)", tiers[0].Tier)
	}
	if last := tiers[len(tiers)-1]; last.Tier != RadiantTier {
		t.Errorf("last tier is %d, want %d (Radiant)", last.Tier, RadiantTier)
	}
	for _, tier := range tiers {
		if tier.Tier == 1 || tier.Tier == 2 {
			t.Errorf("the Unused tier %d was enumerated", tier.Tier)
		}
	}
	if !slices.IsSortedFunc(tiers, func(a, b Tier) int { return a.Tier - b.Tier }) {
		t.Error("tiers are not in ladder order")
	}
}

// An empty catalogue enumerates nothing, and a nil one does not panic. Both
// hand back a slice a caller can range over without checking first.
func TestEnumeratorsOnAnEmptyCatalogue(t *testing.T) {
	for name, cat := range map[string]*Catalogue{"zero": {}, "nil": nil} {
		if got := len(cat.Agents(types.DefaultLocale)); got != 0 {
			t.Errorf("%s catalogue enumerated %d agents", name, got)
		}
		if got := len(cat.Maps(types.DefaultLocale)); got != 0 {
			t.Errorf("%s catalogue enumerated %d maps", name, got)
		}
		if got := len(cat.Tiers(types.DefaultLocale)); got != 0 {
			t.Errorf("%s catalogue enumerated %d tiers", name, got)
		}
		if got := len(cat.PlayerCards()); got != 0 {
			t.Errorf("%s catalogue enumerated %d cards", name, got)
		}
	}
}
