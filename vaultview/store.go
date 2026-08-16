// store.go remembers the last-used vault: one absolute path in a plain
// text file, no format and no dependency.
//
// The file lives at ~/.config/vaultview/vault. $XDG_CONFIG_HOME is
// honoured when set; otherwise the literal ~/.config is used —
// deliberately NOT os.UserConfigDir(), which on macOS resolves to
// ~/Library/Application Support and would put the file where it must not
// be. An absent, empty, or unreadable file reads as "no default".

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// configDir returns the base configuration directory: $XDG_CONFIG_HOME
// when set, else the literal ~/.config. Empty when no home directory
// resolves.
func configDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return x
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config")
}

// storePath returns the vault store file's path, or "" when no
// configuration directory resolves.
func storePath() string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "vaultview", "vault")
}

// LoadStoredVault returns the stored default vault path, or "" when the
// store is absent, empty, or unreadable.
func LoadStoredVault() string {
	p := storePath()
	if p == "" {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveStoredVault writes the vault path as the stored default, creating
// the directory as needed.
func SaveStoredVault(vault string) error {
	p := storePath()
	if p == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(vault+"\n"), 0o644)
}
