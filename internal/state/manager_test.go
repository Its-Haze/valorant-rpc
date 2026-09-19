package state

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

func TestManager_Apply_UpdatesAndNotifies(t *testing.T) {
	m := NewManager(zerolog.Nop())
	updates := m.Updates()

	m.Apply(func(s *State) { s.SessionLoopState = types.SessionLoopInGame })

	if got := m.Get().SessionLoopState; got != types.SessionLoopInGame {
		t.Errorf("SessionLoopState = %q, want INGAME", got)
	}
	select {
	case st := <-updates:
		if st.SessionLoopState != types.SessionLoopInGame {
			t.Errorf("notified state has SessionLoopState = %q", st.SessionLoopState)
		}
	default:
		t.Error("Apply() did not notify Updates()")
	}
}

func TestManager_Apply_NoChangeDoesNotNotify(t *testing.T) {
	m := NewManager(zerolog.Nop())
	updates := m.Updates()
	m.Apply(func(s *State) { s.AccountLevel = 42 })
	<-updates

	m.Apply(func(s *State) { s.AccountLevel = 42 })

	select {
	case <-updates:
		t.Error("an Apply() that changed nothing still notified")
	default:
	}
}

func TestManager_Get_ReturnsACopy(t *testing.T) {
	m := NewManager(zerolog.Nop())
	m.Apply(func(s *State) { s.RiotID = "Haze" })

	got := m.Get()
	got.RiotID = "Someone Else"

	if m.Get().RiotID != "Haze" {
		t.Error("mutating the state Get() returned changed the manager's own state")
	}
}

func TestManager_Subscribe_FansOutToEverySubscriber(t *testing.T) {
	m := NewManager(zerolog.Nop())
	updates := m.Updates()
	subA := m.Subscribe()
	subB := m.Subscribe()

	m.Apply(func(s *State) { s.MapID = "/Game/Maps/Ascent/Ascent" })

	for name, ch := range map[string]<-chan *State{"A": subA, "B": subB} {
		select {
		case st := <-ch:
			if st.MapID != "/Game/Maps/Ascent/Ascent" {
				t.Errorf("subscriber %s got MapID %q", name, st.MapID)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %s received nothing", name)
		}
	}

	select {
	case st := <-updates:
		if st.MapID != "/Game/Maps/Ascent/Ascent" {
			t.Errorf("Updates() got MapID %q", st.MapID)
		}
	default:
		t.Error("Updates() did not receive the change")
	}
}

func TestManager_Subscribe_CoalescesForASlowSubscriber(t *testing.T) {
	m := NewManager(zerolog.Nop())
	sub := m.Subscribe()

	m.Apply(func(s *State) { s.ScoreAlly = 1 })
	m.Apply(func(s *State) { s.ScoreAlly = 2 })
	m.Apply(func(s *State) { s.ScoreAlly = 3 })

	st := <-sub
	if st.ScoreAlly != 3 {
		t.Fatalf("expected the latest coalesced state (ScoreAlly=3), got %d", st.ScoreAlly)
	}
	select {
	case extra := <-sub:
		t.Fatalf("expected only the latest state buffered, also got ScoreAlly=%d", extra.ScoreAlly)
	default:
	}
}

func TestManager_Apply_StampsContextEntryOnlyOnAChange(t *testing.T) {
	m := NewManager(zerolog.Nop())

	m.Apply(func(s *State) { s.SessionLoopState = types.SessionLoopInGame })
	entered := m.Get().ContextEnteredAt
	if entered.IsZero() {
		t.Fatal("entering a new context did not stamp ContextEnteredAt")
	}

	// A score change inside the same context must not restart the timer.
	m.Apply(func(s *State) { s.ScoreAlly = 4 })
	if got := m.Get().ContextEnteredAt; !got.Equal(entered) {
		t.Errorf("ContextEnteredAt moved within one context: %v then %v", entered, got)
	}

	// Windows' clock granularity is coarse enough that two stamps taken in
	// the same tick compare equal, which would pass for the wrong reason.
	time.Sleep(2 * time.Millisecond)

	m.Apply(func(s *State) { s.SessionLoopState = types.SessionLoopMenus })
	if got := m.Get().ContextEnteredAt; !got.After(entered) {
		t.Errorf("ContextEnteredAt = %v, want a later stamp after leaving the match", got)
	}
}

func TestManager_ConcurrentApplyAndRead(t *testing.T) {
	m := NewManager(zerolog.Nop())
	sub := m.Subscribe()

	// A subscriber that never reads would otherwise hide a blocking send.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range sub {
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Apply(func(s *State) { s.ScoreAlly = n*100 + j })
				_ = m.Get().PhaseContext()
			}
		}(i)
	}
	wg.Wait()

	m.Close()
	<-done
}

func TestManager_Close_ClosesEveryChannel(t *testing.T) {
	m := NewManager(zerolog.Nop())
	updates := m.Updates()
	sub := m.Subscribe()

	m.Close()

	if _, open := <-sub; open {
		t.Error("Close() left a subscriber channel open")
	}
	if _, open := <-updates; open {
		t.Error("Close() left the Updates() channel open")
	}

	// Shutdown order is not something a caller can always control, so both
	// of these have to hand back something already closed, not a hang.
	if _, open := <-m.Subscribe(); open {
		t.Error("Subscribe() after Close() returned a live channel")
	}
	if _, open := <-m.Updates(); open {
		t.Error("Updates() after Close() returned a live channel")
	}
	m.Close()
}

func TestManager_Updates_IsNotFedUntilItIsAskedFor(t *testing.T) {
	// A daemon that only subscribes would otherwise fill the shared buffer
	// and then warn about a dropped update on every single change.
	logs := &strings.Builder{}
	m := NewManager(zerolog.New(logs))

	for i := 0; i < updateBuffer+10; i++ {
		m.Apply(func(s *State) { s.ScoreAlly = i })
	}

	if strings.Contains(logs.String(), "dropping update") {
		t.Error("the unread Updates() channel filled up and started warning")
	}
}
