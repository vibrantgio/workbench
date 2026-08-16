package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withStore points the store at a fresh config directory and returns it.
func withStore(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	return cfg
}

func writeStore(t *testing.T, cfg, content string) string {
	t.Helper()
	dir := filepath.Join(cfg, "vaultview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "vault")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadStoredVault(t *testing.T) {
	vault := t.TempDir()
	tests := []struct {
		name  string
		setup func(t *testing.T, cfg string)
		want  string
	}{
		{"absent file reads as no default", func(t *testing.T, cfg string) {}, ""},
		{"empty file reads as no default", func(t *testing.T, cfg string) {
			writeStore(t, cfg, "")
		}, ""},
		{"whitespace-only file reads as no default", func(t *testing.T, cfg string) {
			writeStore(t, cfg, " \n\t\n")
		}, ""},
		{"unreadable file reads as no default", func(t *testing.T, cfg string) {
			p := writeStore(t, cfg, vault+"\n")
			if err := os.Chmod(p, 0o000); err != nil {
				t.Fatal(err)
			}
			if _, err := os.ReadFile(p); err == nil {
				t.Skip("running with permissions that ignore file modes")
			}
		}, ""},
		{"stored path is returned trimmed", func(t *testing.T, cfg string) {
			writeStore(t, cfg, "  "+vault+"\n")
		}, vault},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := withStore(t)
			tc.setup(t, cfg)
			if got := LoadStoredVault(); got != tc.want {
				t.Errorf("LoadStoredVault() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStorePathHonoursXDGThenLiteralConfig(t *testing.T) {
	t.Run("XDG_CONFIG_HOME wins when set", func(t *testing.T) {
		cfg := withStore(t)
		want := filepath.Join(cfg, "vaultview", "vault")
		if got := storePath(); got != want {
			t.Errorf("storePath() = %q, want %q", got, want)
		}
	})
	t.Run("literal ~/.config otherwise — never the platform config dir", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", home)
		want := filepath.Join(home, ".config", "vaultview", "vault")
		if got := storePath(); got != want {
			t.Errorf("storePath() = %q, want %q", got, want)
		}
	})
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	withStore(t)
	vault := t.TempDir()
	if err := SaveStoredVault(vault); err != nil {
		t.Fatalf("SaveStoredVault: %v", err)
	}
	if got := LoadStoredVault(); got != vault {
		t.Errorf("LoadStoredVault() = %q, want %q", got, vault)
	}
	data, err := os.ReadFile(storePath())
	if err != nil {
		t.Fatalf("reading store: %v", err)
	}
	if want := vault + "\n"; string(data) != want {
		t.Errorf("store file = %q, want plain text %q", data, want)
	}
}

func TestResolveVault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("CLI argument wins over the store", func(t *testing.T) {
		cfg := withStore(t)
		writeStore(t, cfg, t.TempDir())
		got, ok := resolveVault([]string{"vaultview", dir})
		if !ok || got != dir {
			t.Errorf("resolveVault = %q, %v; want %q, true", got, ok, dir)
		}
	})
	t.Run("stored default used without an argument", func(t *testing.T) {
		cfg := withStore(t)
		writeStore(t, cfg, dir+"\n")
		got, ok := resolveVault([]string{"vaultview"})
		if !ok || got != dir {
			t.Errorf("resolveVault = %q, %v; want %q, true", got, ok, dir)
		}
	})
	t.Run("stored path that stopped being a directory falls through", func(t *testing.T) {
		cfg := withStore(t)
		writeStore(t, cfg, file+"\n")
		if got, ok := resolveVault([]string{"vaultview"}); ok {
			t.Errorf("resolveVault = %q, true; want fall-through to the picker", got)
		}
	})
	t.Run("stored path that vanished falls through", func(t *testing.T) {
		cfg := withStore(t)
		writeStore(t, cfg, filepath.Join(dir, "gone")+"\n")
		if got, ok := resolveVault([]string{"vaultview"}); ok {
			t.Errorf("resolveVault = %q, true; want fall-through to the picker", got)
		}
	})
	t.Run("no store and no argument falls through", func(t *testing.T) {
		withStore(t)
		if got, ok := resolveVault([]string{"vaultview"}); ok {
			t.Errorf("resolveVault = %q, true; want fall-through to the picker", got)
		}
	})
}
