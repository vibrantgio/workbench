// vaultpanel.go is the platform side of Switch Vault: the folder chooser
// this app asks before it raises one of its own.
//
// Choosing a folder is the platform's open panel where the platform offers
// one — on macOS a sheet on this window, with the Finder's own sidebar,
// search and shortcuts in it — and vaultpicker.go's modal where it does
// not. Both answer the same way: a path opens the vault and the whole
// window follows, and nothing leaves the vault on screen as it was.

package main

import (
	"sync/atomic"

	"gioui.org/app"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/system/openpanel"
)

// folderChooser is the platform's own chooser as this app reaches it:
// whether the platform has one, and the call that presents it and blocks
// until the reader answers.
type folderChooser struct {
	available func() bool
	choose    func(dir string) (path string, ok bool)
}

// platformPanel is the chooser Switch Vault asks first. It is a variable
// because both sides of that branch have to be readable with no window and
// no reader: a test reports no panel to walk the fallback, and records the
// directory the call is reached with to read the other side.
var platformPanel = folderChooser{
	available: openpanel.Available,
	choose:    chooseVaultFolder,
}

// nativeView is the handle of the window's platform view, which is what the
// platform's panel attaches its sheet to. Gio hands it out as a view event;
// an invalid event means the view left its window, so the handle is dropped
// and a panel raised before another arrives stands on its own.
var nativeView atomic.Uintptr

// watchNativeView keeps nativeView on the window's current view. It is the
// window's one subscription to that stream — a second would compete with it
// for the same channel — and it is made before Render so no event is
// evicted before it is read.
func watchNativeView(w *mvu.Window) {
	_ = w.ViewEvents().Subscribe(rx.GoroutineContext(), func(e app.ViewEvent, _ error, done bool) {
		if !done {
			nativeView.Store(viewHandle(e))
		}
	})
}

// chooseVaultFolder presents the platform's panel on this window. It blocks
// until the reader answers it, which is why it runs as a command rather
// than in an update.
func chooseVaultFolder(dir string) (string, bool) {
	return openpanel.ChooseDirectory(nativeView.Load(), dir)
}
