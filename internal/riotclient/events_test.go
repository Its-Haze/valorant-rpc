package riotclient

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/rs/zerolog"
)

// A presence frame as the Riot Client sends it, trimmed to one entry.
const presenceFrame = `[8,"OnJsonApiEvent_chat_v4_presences",{"data":{"presences":[` +
	`{"puuid":"11111111-2222-3333-4444-555555555555","product":"valorant","private":"eyJ9"}` +
	`]},"eventType":"Update","uri":"/chat/v4/presences"}]`

func discardLogger() zerolog.Logger { return zerolog.New(io.Discard) }

func TestParseFrameReadsARecordedPresenceEnvelope(t *testing.T) {
	event, ok := parseFrame([]byte(presenceFrame))
	if !ok {
		t.Fatal("parseFrame rejected a real envelope")
	}

	if event.Name != "OnJsonApiEvent_chat_v4_presences" {
		t.Errorf("Name = %q", event.Name)
	}
	if event.EventType != "Update" {
		t.Errorf("EventType = %q, want Update", event.EventType)
	}
	if event.URI != "/chat/v4/presences" {
		t.Errorf("URI = %q", event.URI)
	}

	var data struct {
		Presences []struct {
			PUUID   string `json:"puuid"`
			Product string `json:"product"`
		} `json:"presences"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("Data is not the payload's data field: %v", err)
	}
	if len(data.Presences) != 1 || data.Presences[0].Product != "valorant" {
		t.Errorf("Data = %s", event.Data)
	}
}

// Every one of these crashes the reference LCU implementation, which
// type-asserts eventType and uri without checking.
func TestParseFrameRejectsMalformedFrames(t *testing.T) {
	cases := map[string]string{
		"not json":              `{{{`,
		"not an array":          `{"opcode":8}`,
		"too short":             `[8,"OnJsonApiEvent_chat_v4_presences"]`,
		"empty array":           `[]`,
		"wrong opcode":          `[5,"OnJsonApiEvent_chat_v4_presences",{"eventType":"Update"}]`,
		"opcode not a number":   `["8","OnJsonApiEvent_chat_v4_presences",{"eventType":"Update"}]`,
		"name not a string":     `[8,42,{"eventType":"Update"}]`,
		"empty name":            `[8,"",{"eventType":"Update"}]`,
		"payload not an object": `[8,"OnJsonApiEvent_chat_v4_presences","Update"]`,
		"eventType not string":  `[8,"OnJsonApiEvent_chat_v4_presences",{"eventType":7,"uri":"/x"}]`,
		"uri not a string":      `[8,"OnJsonApiEvent_chat_v4_presences",{"eventType":"Update","uri":[]}]`,
		"null payload":          `[8,"OnJsonApiEvent_chat_v4_presences",null]`,
	}
	for name, frame := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := parseFrame([]byte(frame)); ok {
				t.Errorf("parseFrame accepted %s", frame)
			}
		})
	}
}

// A payload missing eventType or uri is still a usable event: the fields
// degrade to empty rather than dropping the data.
func TestParseFrameAcceptsAPayloadMissingMetadata(t *testing.T) {
	event, ok := parseFrame([]byte(`[8,"OnJsonApiEvent_chat_v4_presences",{"data":{"a":1}}]`))
	if !ok {
		t.Fatal("parseFrame rejected a payload with only data")
	}
	if event.EventType != "" || event.URI != "" {
		t.Errorf("want empty metadata, got %+v", event)
	}
	if string(event.Data) != `{"a":1}` {
		t.Errorf("Data = %s", event.Data)
	}
}

func TestDispatchRoutesByEventName(t *testing.T) {
	var reg registry
	var wanted, other int

	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { wanted++ })
	reg.add("OnJsonApiEvent_riot_messaging_service_v1_message", func(Event) { other++ })

	reg.dispatch([]byte(presenceFrame), discardLogger())

	if wanted != 1 {
		t.Errorf("presence handler ran %d times, want 1", wanted)
	}
	if other != 0 {
		t.Errorf("unrelated handler ran %d times, want 0", other)
	}
}

func TestDispatchRunsEveryHandlerForOneName(t *testing.T) {
	var reg registry
	var order []int

	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { order = append(order, 1) })
	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { order = append(order, 2) })

	reg.dispatch([]byte(presenceFrame), discardLogger())

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("handlers ran %v, want [1 2] in registration order", order)
	}
}

func TestDispatchDropsMalformedFramesWithoutCallingHandlers(t *testing.T) {
	var reg registry
	called := false
	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { called = true })

	for _, frame := range []string{`{{{`, `[]`, `[8,42,{}]`, `[8,"OnJsonApiEvent_chat_v4_presences"]`} {
		reg.dispatch([]byte(frame), discardLogger())
	}

	if called {
		t.Error("a malformed frame reached a handler")
	}
}

// The subscribe acknowledgement is an empty frame and must not be treated as
// a malformed one.
func TestDispatchIgnoresEmptyFrames(t *testing.T) {
	var reg registry
	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { t.Error("empty frame reached a handler") })

	reg.dispatch(nil, discardLogger())
	reg.dispatch([]byte("  \n"), discardLogger())
}

func TestDispatchIgnoresUnsubscribedNames(t *testing.T) {
	var reg registry
	reg.add("OnJsonApiEvent_chat_v4_presences", func(Event) { t.Error("wrong handler ran") })

	reg.dispatch([]byte(`[8,"OnJsonApiEvent_lol_gameflow_v1_session",{"eventType":"Update"}]`), discardLogger())
}

func TestRegistryNamesAreDeduplicatedAndOrdered(t *testing.T) {
	var reg registry
	reg.add("b", func(Event) {})
	reg.add("a", func(Event) {})
	reg.add("b", func(Event) {})

	names := reg.names()
	if len(names) != 2 || names[0] != "b" || names[1] != "a" {
		t.Errorf("names() = %v, want [b a]", names)
	}
}

func TestTruncateCapsLongFrames(t *testing.T) {
	if got := truncate([]byte("abcdef"), 3); got != "abc..." {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate([]byte("abc"), 3); got != "abc" {
		t.Errorf("truncate = %q", got)
	}
}
