package gamelog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Lines copied from a real ShooterGame.log, 2026-09-19. Wushu is Jett.
const (
	possessed = `[2026.09.19-21.11.36:355][786]LogPlayerController: Warning: [77476] AcknowledgePossession('Wushu_PC_C_2147273058')`
	current   = `[2026.09.19-21.11.19:610][123]LogShooterPlayerController: Warning: Shooter Player Controller received PlayerState. Current character: Default__Wushu_PC_C`
	cleared   = `[2026.09.19-21.30.37:340][244]LogShooterPlayerController: Warning: Shooter Player Controller received PlayerState. Current character: None`
	menu      = `[2026.09.19-20.19.04:066][ 12]LogShooterPlayerController: Warning: Current character: Default__Career_PC_C`
)

func write(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ShooterGame.log")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return path
}

func TestCharacterReadsTheCodename(t *testing.T) {
	r := New(Options{Path: write(t, possessed, current)})

	got, err := r.Character()
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if got != "Wushu" {
		t.Errorf("Character = %q, want Wushu", got)
	}
}

// The log is append-only for a whole session, so an old match's agent sits
// above the current one and only the last line counts.
func TestCharacterPrefersTheLastLine(t *testing.T) {
	r := New(Options{Path: write(t, current, cleared)})

	got, err := r.Character()
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if got != "" {
		t.Errorf("Character = %q, want empty after None", got)
	}
}

// The menus log the same line with a UI class. The reader passes it through
// and the catalogue join rejects it, so nothing here needs a deny list.
func TestCharacterReturnsUIClassesUnfiltered(t *testing.T) {
	r := New(Options{Path: write(t, menu)})

	got, err := r.Character()
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if got != "Career" {
		t.Errorf("Character = %q, want Career", got)
	}
}

func TestCharacterIsEmptyWithNoMatchingLine(t *testing.T) {
	r := New(Options{Path: write(t, "[2026.09.19-21.00.00:000][ 1]LogNothing: nothing of ours")})

	got, err := r.Character()
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if got != "" {
		t.Errorf("Character = %q, want empty", got)
	}
}

func TestCharacterReportsAMissingLog(t *testing.T) {
	r := New(Options{Path: filepath.Join(t.TempDir(), "absent.log")})

	if _, err := r.Character(); !errors.Is(err, ErrNoLog) {
		t.Fatalf("err = %v, want ErrNoLog", err)
	}
}

// Only the tail is read, so a line that fell out of the window is gone
// rather than stale.
func TestCharacterOnlyScansTheTail(t *testing.T) {
	filler := strings.Repeat("[2026.09.19-21.00.00:000][ 1]LogFiller: padding\n", 200)
	path := write(t, current+"\n"+filler)

	r := New(Options{Path: path, Tail: 64})

	got, err := r.Character()
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if got != "" {
		t.Errorf("Character = %q, want empty when the line is out of the tail", got)
	}
}
