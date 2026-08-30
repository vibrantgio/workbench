package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestRosterCoversEveryApp checks that every entry names a directory beside
// this command holding a module of its own, and that every such directory has
// an entry.
func TestRosterCoversEveryApp(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var present []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(e.Name(), "go.mod")); err == nil {
			present = append(present, e.Name())
		}
	}
	listed := make([]string, 0, len(Apps))
	for _, app := range Apps {
		listed = append(listed, app.Dir)
	}
	slices.Sort(present)
	slices.Sort(listed)
	if !slices.Equal(present, listed) {
		t.Errorf("the roster and the repository disagree:\n  beside the command: %v\n  in the roster:      %v", present, listed)
	}
}

// TestRosterNamesAreDistinct guards the Model's key: Name is what a status is
// stored under, so two apps sharing one would share a lifecycle.
func TestRosterNamesAreDistinct(t *testing.T) {
	seen := make(map[string]bool, len(Apps))
	for _, app := range Apps {
		if seen[app.Name] {
			t.Errorf("two apps are both called %q", app.Name)
		}
		seen[app.Name] = true
	}
}

// TestReleasedLaunchNamesTheAppsOwnRelease covers the first case for every
// entry in the roster: a released build asks Go for the app's own latest
// release, by module path, independent of the working directory.
func TestReleasedLaunchNamesTheAppsOwnRelease(t *testing.T) {
	const root = "/somewhere/workbench"
	for _, app := range Apps {
		t.Run(app.Dir, func(t *testing.T) {
			inv := appInvocation(app, "v0.4.2", root)
			want := []string{"run", "github.com/vibrantgio/workbench/" + app.Dir + "@latest"}
			if !slices.Equal(inv.Args, want) {
				t.Errorf("args = %q, want %q", inv.Args, want)
			}
			if inv.Dir != "" {
				t.Errorf("dir = %q, want the launcher's own", inv.Dir)
			}
		})
	}
}

// TestCheckoutLaunchRunsTheAppNextDoor covers the second case for every entry:
// a build from source runs the copy in the checkout, from inside the app's own
// directory so that the app's go.mod anchors the build.
func TestCheckoutLaunchRunsTheAppNextDoor(t *testing.T) {
	const root = "/somewhere/workbench"
	for _, app := range Apps {
		t.Run(app.Dir, func(t *testing.T) {
			inv := appInvocation(app, "(devel)", root)
			want := []string{"run", "."}
			if !slices.Equal(inv.Args, want) {
				t.Errorf("args = %q, want %q", inv.Args, want)
			}
			if got := filepath.Join(root, app.Dir); inv.Dir != got {
				t.Errorf("dir = %q, want %q", inv.Dir, got)
			}
		})
	}
}

// TestReleasedAndCheckoutDiffer pins that the two cases are not the same
// command, so a released build cannot silently run a checkout and a checkout
// cannot silently fetch a release.
func TestReleasedAndCheckoutDiffer(t *testing.T) {
	app := Apps[0]
	if released, checkout := appInvocation(app, "v1.0.0", "/somewhere/workbench"), appInvocation(app, "(devel)", "/somewhere/workbench"); slices.Equal(released.Args, checkout.Args) {
		t.Errorf("both cases build %q", released.Args)
	}
}

func TestIsRelease(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"", false},
		{"(devel)", false},
		{"devel", false},
		{"v0.0.1", true},
		{"v1.2.3", true},
		{"v0.0.0-20260821041500-0123456789ab", true},
	} {
		if got := isRelease(tc.version); got != tc.want {
			t.Errorf("isRelease(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

// TestOwnVersionIsThisBuild pins what the released case is decided from. A
// test binary is built from source, so its own version must read as one.
func TestOwnVersionIsThisBuild(t *testing.T) {
	if v := ownVersion(); isRelease(v) {
		t.Errorf("a test binary reports version %q, which reads as a release", v)
	}
}

// TestLaunchInvocationInThisCheckout runs the resolution for real: a test
// binary is built from source and runs in the checkout, so it is exactly the
// second case, and the answer must point at the app next door.
func TestLaunchInvocationInThisCheckout(t *testing.T) {
	root, err := workbenchRoot()
	if err != nil {
		t.Fatalf("workbenchRoot: %v", err)
	}
	for _, app := range Apps {
		t.Run(app.Dir, func(t *testing.T) {
			inv, err := launchInvocation(app)
			if err != nil {
				t.Fatalf("launchInvocation: %v", err)
			}
			if want := []string{"run", "."}; !slices.Equal(inv.Args, want) {
				t.Errorf("args = %q, want %q", inv.Args, want)
			}
			if want := filepath.Join(root, app.Dir); inv.Dir != want {
				t.Errorf("dir = %q, want %q", inv.Dir, want)
			}
			if _, err := os.Stat(filepath.Join(inv.Dir, "go.mod")); err != nil {
				t.Errorf("nothing to run in %q: %v", inv.Dir, err)
			}
		})
	}
}

// TestWorkbenchRootWalksUpToTheCheckout builds a checkout-shaped tree in a
// temporary directory — a root module and an app module under it — and starts
// inside the app, which is the awkward case: the app's own go.mod is nearer
// and must not answer.
func TestWorkbenchRootWalksUpToTheCheckout(t *testing.T) {
	root := t.TempDir()
	writeModule(t, root, rootModule)
	app := filepath.Join(root, "todos")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	writeModule(t, app, rootModule+"/todos")

	t.Chdir(app)
	got, err := workbenchRoot()
	if err != nil {
		t.Fatalf("workbenchRoot: %v", err)
	}
	if want := resolve(t, root); resolve(t, got) != want {
		t.Errorf("root = %q, want %q", got, want)
	}
}

// TestWorkbenchRootRefusesAnAppAlone confirms the walk cannot mistake an app
// for the checkout: a module named for one of the apps, with nothing above it,
// is not an answer.
func TestWorkbenchRootRefusesAnAppAlone(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, rootModule+"/todos")

	t.Chdir(dir)
	if got, err := workbenchRoot(); err == nil {
		t.Errorf("workbenchRoot found %q, want an error", got)
	}
}

func writeModule(t *testing.T, dir, path string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+path+"\n\ngo 1.25.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// resolve follows symlinks, because a temporary directory on macOS is reached
// by one and os.Getwd reports the other side of it.
func resolve(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}
