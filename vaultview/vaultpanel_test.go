package main

import (
	"reflect"
	"testing"

	"github.com/vibrantgio/theme/system/openpanel"
)

// withoutPlatformPanel puts the app on a platform that offers no open panel
// of its own, which is where the library's modal is the chooser. The tests
// of that modal stand on this rather than on the platform they run on.
func withoutPlatformPanel(t *testing.T) {
	t.Helper()
	usePlatformPanel(t, folderChooser{
		available: func() bool { return false },
		choose: func(string) (string, bool) {
			t.Error("the panel was presented on a platform that reports none")
			return "", false
		},
	})
}

// usePlatformPanel stands the app on the given chooser for one test and
// puts the real one back afterwards.
func usePlatformPanel(t *testing.T, c folderChooser) {
	t.Helper()
	prev := platformPanel
	platformPanel = c
	t.Cleanup(func() { platformPanel = prev })
}

// TestWithNoPanelSwitchVaultOpensTheModal is the fallback: on a platform
// that reports no open panel, Switch Vault raises the library's own dialog,
// seated at the vault on screen, exactly as it did before the platform was
// asked at all.
func TestWithNoPanelSwitchVaultOpensTheModal(t *testing.T) {
	withoutPlatformPanel(t)

	m := Model{Screen: screenVault, Vault: "/vaults/Second Brain"}
	m, cmd := Update(m, SwitchVault{})

	if !m.PickerOpen {
		t.Fatal("no panel exists and Switch Vault raised no dialog; the reader has no chooser at all")
	}
	if m.PickerDir != "/vaults/Second Brain" {
		t.Errorf("the dialog opened at %q, want the open vault", m.PickerDir)
	}
	if m.Screen != screenVault {
		t.Error("the dialog replaced the vault screen; it stands over it")
	}
	// The dialog's rows are listed for it, which is the command the
	// fallback starts and the panel path starts nothing of.
	if msg, err := cmd.First(); err != nil {
		t.Fatalf("the listing command failed: %v", err)
	} else if _, ok := msg.(pickerListed); !ok {
		t.Errorf("the fallback started %T, want the directory listing the dialog shows", msg)
	}
}

// TestWithAPanelSwitchVaultAsksItAtTheOpenVault reads the other side of the
// branch: where the platform has a panel, Switch Vault raises no dialog,
// asks the platform starting at the vault on screen, and opens what comes
// back. The panel itself is not presented — the call is injected — because
// a real one needs a window and a reader to answer it.
func TestWithAPanelSwitchVaultAsksItAtTheOpenVault(t *testing.T) {
	var asked []string
	usePlatformPanel(t, folderChooser{
		available: func() bool { return true },
		choose: func(dir string) (string, bool) {
			asked = append(asked, dir)
			return "/vaults/Other", true
		},
	})

	before := Model{Screen: screenVault, Vault: "/vaults/Second Brain"}
	m, cmd := Update(before, SwitchVault{})

	if m.PickerOpen {
		t.Error("the platform has a panel and the library raised its modal as well")
	}
	if !reflect.DeepEqual(m, before) {
		t.Error("Switch Vault changed the window before the reader answered the panel")
	}

	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the panel command failed: %v", err)
	}
	if got, want := len(asked), 1; got != want {
		t.Fatalf("the panel was asked %d times, want %d", got, want)
	}
	if asked[0] != "/vaults/Second Brain" {
		t.Errorf("the panel opened at %q, want the vault on screen", asked[0])
	}
	if got, want := msg, (OpenVault{Path: "/vaults/Other"}); got != want {
		t.Errorf("the panel's answer posted %v, want %v", got, want)
	}
}

// TestCancellingThePanelPostsNothing requires a cancelled panel to leave
// the window untouched: no message at all, so no update runs and nothing
// the reader had is disturbed.
func TestCancellingThePanelPostsNothing(t *testing.T) {
	usePlatformPanel(t, folderChooser{
		available: func() bool { return true },
		choose:    func(string) (string, bool) { return "", false },
	})

	_, cmd := Update(Model{Screen: screenVault, Vault: "/vaults/Second Brain"}, SwitchVault{})
	if msg, err := cmd.First(); err == nil && msg != nil {
		t.Errorf("a cancelled panel posted %T, want nothing", msg)
	}
}

// TestTheAppAsksTheRealPanel pins the wiring itself: what Switch Vault asks
// is the platform's own chooser, not a stand-in that answers no everywhere
// and quietly leaves macOS on the library's modal.
func TestTheAppAsksTheRealPanel(t *testing.T) {
	if got, want := platformPanel.available(), openpanel.Available(); got != want {
		t.Errorf("Switch Vault reports a panel = %v, the platform reports %v", got, want)
	}
	if platformPanel.choose == nil {
		t.Error("Switch Vault has no call to present the panel with")
	}
}
