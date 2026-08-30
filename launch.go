package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// invocation is the `go` command that starts one app: the arguments after the
// program name, and the directory to run them in — empty meaning wherever
// this command was itself started.
type invocation struct {
	Args []string
	Dir  string
}

// appInvocation builds the command that starts app, given this build's own
// module version and the workbench checkout root (used only by the second
// case).
//
// A released build names the app's own latest release: the apps are separate
// products on separate tags, so what this build was compiled beside says
// nothing about what the app has published since. `go run <module>@latest`
// also ignores whatever module the user is standing in, so it runs the same
// from anywhere.
//
// A build from source has no release to name, so it runs the app out of the
// checkout as `go run .` inside the app's own directory. That resolves both
// under the workspace above the checkout and in a bare clone of this
// repository, because the app's own go.mod anchors the build either way.
// Naming the app from the root as `./<dir>` would resolve only under the
// workspace: a nested module is not part of its parent, so the root module
// does not contain the package.
func appInvocation(app App, version, root string) invocation {
	if isRelease(version) {
		return invocation{Args: []string{"run", app.Module() + "@latest"}}
	}
	return invocation{Args: []string{"run", "."}, Dir: filepath.Join(root, app.Dir)}
}

// launchInvocation resolves the command for app against this build's own
// version, locating the checkout only in the case that needs one.
func launchInvocation(app App) (invocation, error) {
	version := ownVersion()
	if isRelease(version) {
		return appInvocation(app, version, ""), nil
	}
	root, err := workbenchRoot()
	if err != nil {
		return invocation{}, err
	}
	return appInvocation(app, version, root), nil
}

// isRelease reports whether version — this build's own, as runtime/debug
// states it — names something published. Go writes "(devel)" for a build made
// from source and leaves it empty when build info was stripped; neither can
// be handed to @latest.
func isRelease(version string) bool {
	switch version {
	case "", "(devel)", "devel":
		return false
	}
	return true
}

// ownVersion is this build's own module version, or "" when the binary
// carries no build information at all.
func ownVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// workbenchRoot locates the checkout by walking up from the working directory
// to the first directory whose go.mod declares this command's module. That is
// the repository root and nothing above or below it: each app's go.mod
// declares a module of its own, so none of them can answer to this.
func workbenchRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if declaresRootModule(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no workbench checkout above %s (start it from one)", dir)
		}
		dir = parent
	}
}

// declaresRootModule reports whether the go.mod at path names rootModule.
func declaresRootModule(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for line := range strings.Lines(string(data)) {
		if strings.TrimSpace(line) == "module "+rootModule {
			return true
		}
	}
	return false
}
