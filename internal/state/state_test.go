package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

func TestState_PhaseContext(t *testing.T) {
	tests := []struct {
		name  string
		loop  types.SessionLoopState
		party types.PartyState
		want  types.PresenceContext
	}{
		{"menus and default", types.SessionLoopMenus, types.PartyDefault, types.ContextInClient},
		{"menus and matchmaking", types.SessionLoopMenus, types.PartyMatchmaking, types.ContextInQueue},
		{"menus and custom setup", types.SessionLoopMenus, types.PartyCustomGameSetup, types.ContextCustomGame},
		{"pregame outranks party state", types.SessionLoopPregame, types.PartyMatchmaking, types.ContextAgentSelect},
		{"ingame outranks party state", types.SessionLoopInGame, types.PartyCustomGameSetup, types.ContextInMatch},
		{"empty everything", "", "", types.ContextInClient},
		// The party state keeps reporting MATCHMAKING through a whole match,
		// so it is only trustworthy once the loop state says MENUS.
		{"unknown loop state with a known party state", "LOBBY", types.PartyMatchmaking, types.ContextInClient},
		{"missing loop state still trusts the party state", "", types.PartyMatchmaking, types.ContextInQueue},
		{"unknown loop state and unknown party state", "LOBBY", "SOMETHING", types.ContextInClient},
		{"menus and unknown party state", types.SessionLoopMenus, "SOMETHING", types.ContextInClient},
		{"lowercase loop state", "ingame", "", types.ContextInMatch},
		{"lowercase party state", "menus", "matchmaking", types.ContextInQueue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &State{SessionLoopState: tt.loop, PartyState: tt.party}
			if got := s.PhaseContext(); got != tt.want {
				t.Errorf("PhaseContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestState_IsRange(t *testing.T) {
	tests := []struct {
		flow string
		want bool
	}{
		{types.ProvisioningFlowShootingRange, true},
		{"shootingrange", true},
		{types.ProvisioningFlowMatchmaking, false},
		{"", false},
	}

	for _, tt := range tests {
		s := &State{ProvisioningFlow: tt.flow}
		if got := s.IsRange(); got != tt.want {
			t.Errorf("IsRange() with flow %q = %v, want %v", tt.flow, got, tt.want)
		}
	}

	// The range is a variant of being in a match, never a context of its own.
	s := &State{SessionLoopState: types.SessionLoopInGame, ProvisioningFlow: types.ProvisioningFlowShootingRange}
	if got := s.PhaseContext(); got != types.ContextInMatch {
		t.Errorf("PhaseContext() in the range = %q, want %q", got, types.ContextInMatch)
	}
}

// Every field has to count towards equality, or a change to it never reaches
// Discord. Reflection covers fields added later without touching this test.
func TestState_Equals_CoversEveryField(t *testing.T) {
	base := &State{}
	value := reflect.ValueOf(base).Elem()

	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		other := base.Copy()
		mutated := reflect.ValueOf(other).Elem().Field(i)

		switch mutated.Kind() {
		case reflect.String:
			mutated.SetString("changed")
		case reflect.Int:
			mutated.SetInt(7)
		case reflect.Bool:
			mutated.SetBool(true)
		default:
			if mutated.Type() == reflect.TypeOf(time.Time{}) {
				mutated.Set(reflect.ValueOf(time.Unix(1700000000, 0).UTC()))
				break
			}
			t.Fatalf("field %s has kind %s, which this test does not know how to change", name, mutated.Kind())
		}

		if base.Equals(other) {
			t.Errorf("Equals() = true for states differing only in %s", name)
		}
	}
}

func TestState_Equals_NilAndIdentical(t *testing.T) {
	s := NewState()
	if s.Equals(nil) {
		t.Error("Equals(nil) = true")
	}
	if !s.Equals(s.Copy()) {
		t.Error("Equals() = false for a state and its own copy")
	}
}

func TestState_Copy_IsIndependent(t *testing.T) {
	s := NewState()
	s.RiotID = "Haze"

	c := s.Copy()
	c.RiotID = "Someone Else"

	if s.RiotID != "Haze" {
		t.Errorf("mutating a copy changed the original: RiotID = %q", s.RiotID)
	}
	if (*State)(nil).Copy() != nil {
		t.Error("Copy() on a nil state should stay nil")
	}
}
