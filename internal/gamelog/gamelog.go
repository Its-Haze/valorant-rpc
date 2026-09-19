// Package gamelog reads the agent the local player is on out of Valorant's
// own log file, which is the only local source for it.
package gamelog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNoLog reports a log file that is not there, which is every moment
// before Valorant has been launched at least once.
var ErrNoLog = errors.New("gamelog: no Valorant log file")

// tailBytes is how much of the end of the log is scanned. The file passed
// 1.5MB in a three-hour session, and the possession line has to stay inside
// the window for the whole of a long one.
const tailBytes = 1 << 20

// noCharacter is what the game logs between matches, and it is the reason
// the last line matters rather than the last match of the class pattern.
const noCharacter = "None"

// characterRe matches the line the player controller writes on possession.
// Both spellings appear: the class default object and the spawned pawn.
var characterRe = regexp.MustCompile(`Current character: (?:Default__)?([A-Za-z0-9]+)(?:_PC_C|\b)`)

// Reader reads one fact out of the log: the codename of the agent the local
// player currently controls. It holds no state between reads.
type Reader struct {
	path string
	tail int64
}

// Options configures a Reader. The zero value reads the real log.
type Options struct {
	// Path overrides the log's location, for tests.
	Path string
	// Tail overrides how many bytes of the end are scanned.
	Tail int64
}

// New builds a Reader. It touches no files, so it is safe to build before
// Valorant has ever run.
func New(opts Options) *Reader {
	r := &Reader{path: opts.Path, tail: opts.Tail}
	if r.path == "" {
		r.path = DefaultPath()
	}
	if r.tail <= 0 {
		r.tail = tailBytes
	}
	return r
}

// DefaultPath is where Valorant writes its log. It is the same file the
// client version is scraped from.
func DefaultPath() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return ""
	}
	return filepath.Join(local, "VALORANT", "Saved", "Logs", "ShooterGame.log")
}

// Character returns the codename of the agent the player is on, empty when
// they are not in one. The codename is valorant-api's developerName.
func (r *Reader) Character() (string, error) {
	blob, err := r.readTail()
	if err != nil {
		return "", err
	}

	// The log is append-only for a whole session, so an earlier match's agent
	// sits above this one. Only the last line speaks for now.
	matches := characterRe.FindAllStringSubmatch(string(blob), -1)
	if len(matches) == 0 {
		return "", nil
	}

	name := matches[len(matches)-1][1]
	if strings.EqualFold(name, noCharacter) {
		return "", nil
	}
	return name, nil
}

// readTail reads the last stretch of the log. The game holds the file open
// while it runs, which Windows allows alongside a shared read.
func (r *Reader) readTail() ([]byte, error) {
	if r.path == "" {
		return nil, ErrNoLog
	}

	file, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoLog
		}
		return nil, fmt.Errorf("gamelog: opening %s: %w", r.path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("gamelog: reading the size of %s: %w", r.path, err)
	}

	size, offset := info.Size(), int64(0)
	if size > r.tail {
		offset = size - r.tail
	}

	blob, err := io.ReadAll(io.NewSectionReader(file, offset, size-offset))
	if err != nil {
		return nil, fmt.Errorf("gamelog: reading %s: %w", r.path, err)
	}
	return blob, nil
}
