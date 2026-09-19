package riotchat

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ErrNoPresence reports a payload that carries no Valorant presence for the
// local player. Routine: the list is every friend, and often none of them.
var ErrNoPresence = errors.New("riotchat: no valorant presence for this player")

// queueEntryLayout is Riot's timestamp format, always UTC.
const queueEntryLayout = "2006.01.02-15.04.05"

// Decode picks the local player's Valorant entry out of a /chat/v4/presences
// payload and normalizes it. The same payload arrives over HTTP and over the
// websocket, so this is the only entry point either needs.
func Decode(payload []byte, puuid string, logger zerolog.Logger) (Presence, error) {
	// An event can arrive with no data at all. That is nothing of ours, not
	// a decoding failure worth logging above debug.
	if len(payload) == 0 {
		return Presence{}, ErrNoPresence
	}

	var envelope struct {
		Presences []struct {
			PUUID    string `json:"puuid"`
			Product  string `json:"product"`
			GameName string `json:"game_name"`
			GameTag  string `json:"game_tag"`
			Private  string `json:"private"`
		} `json:"presences"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return Presence{}, fmt.Errorf("riotchat: decoding the presence payload: %w", err)
	}

	// A malformed entry must not hide a good duplicate, but it is also the
	// shape change worth reporting when nothing else decodes.
	var malformed error

	for _, entry := range envelope.Presences {
		if entry.PUUID != puuid || !strings.EqualFold(entry.Product, ProductValorant) {
			continue
		}

		// Riot publishes our own entry with no private blob on login and on
		// going away. Nothing of ours yet, not a failure.
		if entry.Private == "" {
			logger.Debug().Msg("Our presence entry carries no private blob yet")
			continue
		}

		blob, err := decodeBase64(entry.Private)
		if err != nil {
			malformed = err
			continue
		}

		presence, err := decodePrivate(blob, logger)
		if err != nil {
			malformed = err
			continue
		}
		presence.PUUID = entry.PUUID
		presence.GameName = entry.GameName
		presence.Tagline = entry.GameTag
		return presence, nil
	}

	if malformed != nil {
		return Presence{}, malformed
	}
	return Presence{}, ErrNoPresence
}

// decodeBase64 accepts Riot's padded blob and an unpadded one.
func decodeBase64(private string) ([]byte, error) {
	blob, err := base64.StdEncoding.DecodeString(private)
	if err == nil {
		return blob, nil
	}
	if raw, rawErr := base64.RawStdEncoding.DecodeString(private); rawErr == nil {
		return raw, nil
	}
	return nil, fmt.Errorf("riotchat: decoding the private blob: %w", err)
}

// decodePrivate normalizes the private blob. Riot is migrating this payload
// between a flat shape and one nesting the match and party fields, so every
// field is looked up in the nested containers before the top level.
func decodePrivate(blob []byte, logger zerolog.Logger) (Presence, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(blob, &top); err != nil {
		return Presence{}, fmt.Errorf("riotchat: parsing the private blob: %w", err)
	}

	f := fields{logger: logger}
	f.push(nested(top, "matchPresenceData", logger))
	f.push(nested(top, "partyPresenceData", logger))
	f.push(top)

	return Presence{
		SessionLoopState:    f.str("sessionLoopState"),
		PartyState:          f.str("partyState"),
		MatchMap:            f.str("matchMap"),
		QueueID:             f.str("queueId"),
		ProvisioningFlow:    f.str("provisioningFlow"),
		ScoreAllyTeam:       f.num("partyOwnerMatchScoreAllyTeam"),
		ScoreEnemyTeam:      f.num("partyOwnerMatchScoreEnemyTeam"),
		PartySize:           f.num("partySize"),
		MaxPartySize:        f.num("maxPartySize"),
		PartyAccessibility:  f.str("partyAccessibility"),
		PartyID:             f.str("partyId"),
		CompetitiveTier:     f.num("competitiveTier"),
		LeaderboardPosition: f.num("leaderboardPosition"),
		AccountLevel:        f.num("accountLevel"),
		IsIdle:              f.flag("isIdle"),
		QueueEntryTime:      f.timestamp("queueEntryTime"),
	}, nil
}

// nested returns one of the containers of the nested shape, or nil when the
// payload is flat.
func nested(top map[string]json.RawMessage, key string, logger zerolog.Logger) map[string]json.RawMessage {
	raw, ok := top[key]
	if !ok {
		return nil
	}

	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inner); err != nil {
		logger.Debug().Err(err).Str("field", key).Msg("Ignored an unreadable presence container")
		return nil
	}
	return inner
}

// fields reads one value out of the first scope that carries it. Scopes are
// pushed highest precedence first, so nested containers win over the top level.
type fields struct {
	scopes []map[string]json.RawMessage
	logger zerolog.Logger
}

func (f *fields) push(scope map[string]json.RawMessage) {
	if scope != nil {
		f.scopes = append(f.scopes, scope)
	}
}

func (f *fields) raw(key string) (json.RawMessage, bool) {
	for _, scope := range f.scopes {
		if raw, ok := scope[key]; ok {
			return raw, true
		}
	}
	f.logger.Debug().Str("field", key).Msg("Presence field is missing, using the zero value")
	return nil, false
}

// read unmarshals one field into dst. A field Riot has retyped degrades to
// the zero value at debug rather than failing the whole presence.
func read[T any](f *fields, key string) T {
	var value T

	raw, ok := f.raw(key)
	if !ok {
		return value
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		f.logger.Debug().Err(err).Str("field", key).Msg("Presence field has an unexpected type, using the zero value")
		var zero T
		return zero
	}
	return value
}

func (f *fields) str(key string) string { return read[string](f, key) }
func (f *fields) flag(key string) bool  { return read[bool](f, key) }

// num accepts any JSON number. Riot sends integers today, but a count that
// arrives as 5.0 should not wipe the field.
func (f *fields) num(key string) int {
	raw, ok := f.raw(key)
	if !ok {
		return 0
	}

	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		if whole, err := number.Int64(); err == nil {
			return int(whole)
		}
		if fraction, err := number.Float64(); err == nil {
			return int(fraction)
		}
	}

	f.logger.Debug().Str("field", key).Msg("Presence field is not a number, using the zero value")
	return 0
}

// timestamp parses Riot's 2006.01.02-15.04.05 format. An idle player sends
// the year-one value, which parses straight to the zero time.
func (f *fields) timestamp(key string) time.Time {
	value := f.str(key)
	if value == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(queueEntryLayout, value)
	if err != nil {
		f.logger.Debug().Err(err).Str("field", key).Str("value", value).Msg("Presence timestamp is unparseable, using the zero time")
		return time.Time{}
	}
	return parsed
}
