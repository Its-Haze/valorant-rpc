package riotchat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const selfPUUID = "11111111-2222-3333-4444-555555555555"

func discardLogger() zerolog.Logger { return zerolog.New(io.Discard) }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	blob, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return blob
}

// entry is one presence as the Riot Client publishes it. private holds the
// already-encoded blob, so a test can hand it something undecodable.
type entry struct {
	PUUID    string `json:"puuid"`
	Product  string `json:"product"`
	GameName string `json:"game_name"`
	GameTag  string `json:"game_tag"`
	Private  string `json:"private"`
}

func envelope(t *testing.T, entries ...entry) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"presences": entries})
	if err != nil {
		t.Fatalf("building an envelope: %v", err)
	}
	return payload
}

// valorantEntry wraps a fixture as our own Valorant presence.
func valorantEntry(t *testing.T, fixture string) entry {
	t.Helper()

	return entry{
		PUUID:    selfPUUID,
		Product:  ProductValorant,
		GameName: "Haze",
		GameTag:  "EUW",
		Private:  base64.StdEncoding.EncodeToString(readFixture(t, fixture)),
	}
}

func wantFlatPresence() Presence {
	return Presence{
		PUUID:               selfPUUID,
		GameName:            "Haze",
		Tagline:             "EUW",
		PlayerCardID:        "c2e6c2a0-0000-4000-8000-000000000001",
		SessionLoopState:    "INGAME",
		PartyState:          "DEFAULT",
		MatchMap:            "/Game/Maps/Ascent/Ascent",
		QueueID:             "competitive",
		ScoreAllyTeam:       9,
		ScoreEnemyTeam:      4,
		GameScoreType:       "Rounds",
		PartySize:           2,
		MaxPartySize:        5,
		PartyAccessibility:  "CLOSED",
		PartyID:             "9f9b1a30-1111-4a2b-9d4c-0d1f2e3a4b5c",
		CompetitiveTier:     21,
		LeaderboardPosition: 0,
		AccountLevel:        214,
		IsIdle:              false,
		ProvisioningFlow:    "Matchmaking",
		QueueEntryTime:      time.Date(2026, 9, 19, 14, 32, 7, 0, time.UTC),
	}
}

func TestBothPayloadShapesDecodeToTheSamePresence(t *testing.T) {
	want := wantFlatPresence()

	for _, fixture := range []string{"private_flat.json", "private_nested.json"} {
		t.Run(fixture, func(t *testing.T) {
			got, err := Decode(envelope(t, valorantEntry(t, fixture)), selfPUUID, discardLogger())
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got != want {
				t.Errorf("presence mismatch\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

func TestDecodeSkipsEveryoneElse(t *testing.T) {
	league := entry{
		PUUID:    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Product:  "league_of_legends",
		GameName: "Friend",
		GameTag:  "EUW",
		Private:  string(readFixture(t, "private_league.json")),
	}
	stranger := valorantEntry(t, "private_flat.json")
	stranger.PUUID = "99999999-9999-9999-9999-999999999999"

	mine := valorantEntry(t, "private_flat.json")

	got, err := Decode(envelope(t, league, stranger, mine), selfPUUID, discardLogger())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != wantFlatPresence() {
		t.Errorf("picked the wrong entry: %+v", got)
	}
}

// A League entry under our own puuid must not reach the decoder: its private
// is plain JSON with a completely different shape.
func TestDecodeIgnoresOurOwnNonValorantProducts(t *testing.T) {
	league := entry{
		PUUID:   selfPUUID,
		Product: "league_of_legends",
		Private: string(readFixture(t, "private_league.json")),
	}

	_, err := Decode(envelope(t, league), selfPUUID, discardLogger())
	if !errors.Is(err, ErrNoPresence) {
		t.Fatalf("err = %v, want ErrNoPresence", err)
	}
}

func TestDecodeReportsATruncatedPrivateBlob(t *testing.T) {
	broken := valorantEntry(t, "private_flat.json")
	broken.Private = broken.Private[:len(broken.Private)/2] + "="

	if _, err := Decode(envelope(t, broken), selfPUUID, discardLogger()); err == nil {
		t.Fatal("a truncated blob decoded without error")
	}
}

// Riot publishes our own entry with no private blob on login and on going
// away. Nothing of ours yet, so it must not reach the caller as a failure.
func TestDecodeTreatsAnEmptyPrivateBlobAsNoPresence(t *testing.T) {
	blank := valorantEntry(t, "private_flat.json")
	blank.Private = ""

	if _, err := Decode(envelope(t, blank), selfPUUID, discardLogger()); !errors.Is(err, ErrNoPresence) {
		t.Fatalf("err = %v, want ErrNoPresence", err)
	}
}

func TestDecodeKeepsLookingPastAMalformedEntry(t *testing.T) {
	broken := valorantEntry(t, "private_flat.json")
	broken.Private = "!!!not base64!!!"
	mine := valorantEntry(t, "private_flat.json")

	got, err := Decode(envelope(t, broken, mine), selfPUUID, discardLogger())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != wantFlatPresence() {
		t.Errorf("a malformed duplicate hid the good entry: %+v", got)
	}
}

func TestDecodeReportsAnEmptyPresenceList(t *testing.T) {
	if _, err := Decode([]byte(`{"presences":[]}`), selfPUUID, discardLogger()); !errors.Is(err, ErrNoPresence) {
		t.Fatalf("err = %v, want ErrNoPresence", err)
	}
}

func TestDecodeRejectsAnUnparseablePayload(t *testing.T) {
	if _, err := Decode([]byte(`not json`), selfPUUID, discardLogger()); err == nil {
		t.Fatal("garbage decoded without error")
	}
}

func TestMissingAndMistypedFieldsDegradeToZero(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)

	blob := `{"sessionLoopState":"MENUS","partySize":"two","queueEntryTime":"whenever"}`
	broken := entry{
		PUUID:   selfPUUID,
		Product: ProductValorant,
		Private: base64.StdEncoding.EncodeToString([]byte(blob)),
	}

	got, err := Decode(envelope(t, broken), selfPUUID, logger)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.SessionLoopState != "MENUS" {
		t.Errorf("SessionLoopState = %q", got.SessionLoopState)
	}
	if got.PartySize != 0 {
		t.Errorf("PartySize = %d, want the zero value", got.PartySize)
	}
	if !got.QueueEntryTime.IsZero() {
		t.Errorf("QueueEntryTime = %v, want the zero time", got.QueueEntryTime)
	}
	if !strings.Contains(logs.String(), `"level":"debug"`) {
		t.Errorf("degraded fields logged at the wrong level: %s", logs.String())
	}
	if strings.Contains(logs.String(), `"level":"info"`) || strings.Contains(logs.String(), `"level":"warn"`) {
		t.Errorf("degraded fields logged above debug: %s", logs.String())
	}
}

func TestQueueEntryTimeZeroValueStaysZero(t *testing.T) {
	blob := `{"queueEntryTime":"0001.01.01-00.00.00"}`
	e := entry{PUUID: selfPUUID, Product: ProductValorant, Private: base64.StdEncoding.EncodeToString([]byte(blob))}

	got, err := Decode(envelope(t, e), selfPUUID, discardLogger())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.QueueEntryTime.IsZero() {
		t.Errorf("QueueEntryTime = %v, want the zero time", got.QueueEntryTime)
	}
}

func TestProductMatchIsCaseInsensitive(t *testing.T) {
	e := valorantEntry(t, "private_flat.json")
	e.Product = "VALORANT"

	if _, err := Decode(envelope(t, e), selfPUUID, discardLogger()); err != nil {
		t.Fatalf("Decode: %v", err)
	}
}

// Riot pads its base64, but an unpadded blob is cheap to accept and costs
// nothing if the padding ever goes away.
func TestUnpaddedBase64Decodes(t *testing.T) {
	e := valorantEntry(t, "private_flat.json")
	e.Private = strings.TrimRight(e.Private, "=")

	if _, err := Decode(envelope(t, e), selfPUUID, discardLogger()); err != nil {
		t.Fatalf("Decode: %v", err)
	}
}

func TestNumericFieldsAcceptAFloat(t *testing.T) {
	blob := `{"partySize":5.0,"accountLevel":214}`
	e := entry{PUUID: selfPUUID, Product: ProductValorant, Private: base64.StdEncoding.EncodeToString([]byte(blob))}

	got, err := Decode(envelope(t, e), selfPUUID, discardLogger())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.PartySize != 5 || got.AccountLevel != 214 {
		t.Errorf("PartySize = %d, AccountLevel = %d", got.PartySize, got.AccountLevel)
	}
}

func TestDecodeTreatsAnEmptyPayloadAsNoPresence(t *testing.T) {
	if _, err := Decode(nil, selfPUUID, discardLogger()); !errors.Is(err, ErrNoPresence) {
		t.Fatalf("err = %v, want ErrNoPresence", err)
	}
}
