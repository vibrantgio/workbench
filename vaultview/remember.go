// remember.go keeps the arrangement inside the window across launches:
// how wide the rail pane stands down the leading edge and how wide the
// aside stands down the trailing one. The window's own frame — its size
// and where it is on the desktop — is the runtime's to keep and nothing
// here touches it; the two files sit side by side in this application's
// directory under the OS config directory, each damaged or missing on its
// own without costing the other.
//
// A width is written after it stands still rather than once per frame, so
// one drag of a boundary produces one write, and whatever is still pending
// is written as the window goes away.

package main

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/unit"
)

// columnWidths is how wide each of the window's two side columns stands,
// in device-independent pixels — the unit that makes a kept width the same
// physical measure on a screen of a different pixel density.
type columnWidths struct {
	Rail  unit.Dp `json:"rail"`
	Aside unit.Dp `json:"aside"`
}

// widthFile is the name the arrangement is kept under, inside the
// application's own directory in the OS config directory:
//
//   - darwin:  ~/Library/Application Support/vaultview/layout.json
//   - linux:   $XDG_CONFIG_HOME/vaultview/layout.json (or ~/.config/...)
//   - windows: %AppData%\vaultview\layout.json
const widthFile = "layout.json"

// widthDelay is how long a width must stand still before it is written. It
// is the delay the window's own frame is kept on: dragging a boundary and
// dragging the window's edge are the same gesture at the same speed, so the
// two files settle together rather than one trailing the other.
const widthDelay = 500 * time.Millisecond

// The rail's bounds, the counterpart of the aside's. Below the minimum the
// column is narrower than an indented folder name, so the tree it holds
// shows as ellipses; above the maximum the rail is no longer a rail beside
// the note but a second column competing with it. What is left of the
// window is not this clamp's business — a rail wider than half the window
// is cut to half by the pane pattern, whatever these bounds allow.
const (
	railMinWidthDp = 160
	railMaxWidthDp = 480
)

// defaultWidths is the arrangement a window opens with when nothing was
// kept, which is what this window laid out before it kept anything.
func defaultWidths() columnWidths {
	return columnWidths{Rail: treeWidthDp, Aside: frameAsideDp}
}

// clamped answers the nearest arrangement this window can lay out. What
// comes out of the file is a number a text editor can put there, and a
// window the reader cannot use is not worth obeying a file for, so
// everything arriving from outside passes through here.
func (w columnWidths) clamped() columnWidths {
	return columnWidths{Rail: clampRail(w.Rail), Aside: clampAside(w.Aside)}
}

// plausible reports whether w describes two real widths. A NaN answers
// false to every comparison a clamp makes and would come out of one
// untouched, so it is caught before the clamp rather than inside it.
func (w columnWidths) plausible() bool {
	for _, v := range []unit.Dp{w.Rail, w.Aside} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

func clampRail(w unit.Dp) unit.Dp {
	if w < railMinWidthDp {
		return railMinWidthDp
	}
	if w > railMaxWidthDp {
		return railMaxWidthDp
	}
	return w
}

// loadWidths reads one arrangement from path. Every way of having nothing
// to apply — no file, an unreadable one, damaged JSON, a width that is not
// a number — answers the same false, because they all mean the same thing
// to the caller: open on the defaults.
func loadWidths(path string) (columnWidths, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return columnWidths{}, false
	}
	var w columnWidths
	if err := json.Unmarshal(data, &w); err != nil {
		return columnWidths{}, false
	}
	if !w.plausible() {
		return columnWidths{}, false
	}
	return w, true
}

// saveWidths writes w to path, creating the intermediate directories.
func saveWidths(path string, w columnWidths) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// columnMemory holds the window's arrangement between the frames that
// report it and the file it eventually lands in.
//
// Every method takes a nil receiver, because a frame laid out for
// measurement rather than for a reader has no memory behind it — the
// stored renders above all, which arrange a window nobody is sitting at
// and must not write over what a real one kept.
type columnMemory struct {
	// saved is the arrangement the window opens with: what was kept, where
	// something was kept that can be laid out, and the defaults otherwise.
	// It is read once at construction and never changes.
	saved columnWidths

	path  string
	delay time.Duration
	// write is the one seam this type has. It is a field so a test can
	// count the writes a burst produces without going near a file system.
	write func(columnWidths) error

	mu sync.Mutex
	// current is the last arrangement handed in, the opening one included;
	// pending says whether it still has to be written. have separates the
	// opening arrangement — which is either what the file holds or the
	// defaults, and neither is news — from every later change.
	current columnWidths
	have    bool
	pending bool
	timer   *time.Timer
}

// rememberWidths opens the arrangement kept for appName. An error means
// the OS could not say where its config directory is, and the nil memory
// it comes with is a working one: the window opens on the defaults and
// keeps nothing.
func rememberWidths(appName string) (*columnMemory, error) {
	if appName == "" {
		return nil, errors.New("vaultview: remembering widths needs an application name")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return openWidths(filepath.Join(dir, appName, widthFile), widthDelay), nil
}

// openWidths reads path and returns the memory a window arranges itself
// from and writes back through.
func openWidths(path string, delay time.Duration) *columnMemory {
	saved := defaultWidths()
	if kept, ok := loadWidths(path); ok {
		saved = kept.clamped()
	}
	m := &columnMemory{saved: saved, path: path, delay: delay}
	m.write = func(w columnWidths) error { return saveWidths(m.path, w) }
	return m
}

// widths is the arrangement the window opens with, the defaults where
// there is no memory at all.
func (m *columnMemory) widths() columnWidths {
	if m == nil {
		return defaultWidths()
	}
	return m.saved
}

// record takes in one laid-out arrangement. The first writes nothing: it
// is what the window opened with. An arrangement equal to the last one is
// likewise nothing to write, which is what most frames hand in, since a
// frame is laid out whether or not a boundary moved in it.
//
// Anything else restarts the delay, so a drag collapses into the one write
// that follows it.
func (m *columnMemory) record(w columnWidths) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.have {
		m.current, m.have = w, true
		return
	}
	if w == m.current {
		return
	}
	m.current = w
	m.pending = true
	if m.timer == nil {
		m.timer = time.AfterFunc(m.delay, m.flush)
		return
	}
	m.timer.Reset(m.delay)
}

// flush writes what is pending, if anything. It runs on the timer's own
// goroutine and, through [columnMemory.stop], on the render goroutine as
// the window closes; a failed write is dropped rather than retried,
// because an arrangement nobody could store is not worth a second window's
// worth of attention.
func (m *columnMemory) flush() {
	if m == nil {
		return
	}
	m.mu.Lock()
	pending, w := m.pending, m.current
	m.pending = false
	m.mu.Unlock()
	if !pending {
		return
	}
	_ = m.write(w)
}

// stop writes whatever is pending and stops the timer. The window is going
// away, and a boundary the reader moved inside the delay is exactly the
// change they most expect to find again.
func (m *columnMemory) stop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.mu.Unlock()
	m.flush()
}
