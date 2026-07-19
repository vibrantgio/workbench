package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
)

type Model struct {
	DataDir string
	// AuthToken is the OPENAI_API_KEY environment value; it only seeds the
	// first provider when the loaded config has none (first run, or a
	// config predating provider support).
	AuthToken   string
	CurrentChat Chat
	ChatList    ChatList

	// Providers is the live API endpoint catalogue; DefaultProvider and
	// DefaultModel name the model prompts use unless the chat overrides
	// them. The settings modal edits a DRAFT copy (Settings.Draft) that
	// only replaces these on SaveSettings.
	Providers       []Provider
	DefaultProvider string
	DefaultModel    string

	// Settings drives the settings modal; the zero value means closed.
	Settings SettingsState

	// ModelMenu is whether the chat header's model picker is open.
	ModelMenu bool

	// Pending is the delete currently advertised by the undo bar; the zero
	// value means the bar is hidden. Its ConfirmDelete timer only HIDES the
	// bar — the chat stays undoable via Trash for the whole session.
	Pending PendingDelete
	// Trash is the session's undo stack, most recent last. Deleted chats'
	// history files live in ChatDir()/.trash until undone; leftovers are
	// restored into the chat list on next startup.
	Trash []PendingDelete
	// DeleteGen counts deletes monotonically and never resets, so a stale
	// ConfirmDelete timer from an undone delete can never hide the bar of a
	// later one that reused its slot.
	DeleteGen int

	// Rename is the chat whose rename modal is open; the zero value means
	// closed.
	Rename RenameState

	// Streams tracks in-flight completions by stream id. While a stream's
	// chat is current its deltas apply to CurrentChat.History and the entry
	// holds no buffer; when the user switches away the visible history is
	// stashed into the entry and deltas accumulate there until the stream
	// finishes and saves to ITS chat's file.
	Streams map[int]StreamState
	// NextStream issues stream ids; monotonic, never reused.
	NextStream int

	// SidebarRatio is the split-pane position (0 = use the default);
	// SidebarCollapsed shrinks the sidebar to an icon rail.
	SidebarRatio     float32
	SidebarCollapsed bool
}

// Sidebar geometry: the default split position, the ratio the collapsed
// rail sits at (the split pane's minimum), and the width below which the
// sidebar renders as a rail.
const (
	DefaultSidebarRatio = 0.22
	CollapsedRatio      = 0.05
	RailThresholdRatio  = 0.08
)

// EffectiveRatio is the split-pane position the view renders.
func (model Model) EffectiveRatio() float32 {
	if model.SidebarCollapsed {
		return CollapsedRatio
	}
	if model.SidebarRatio == 0 {
		return DefaultSidebarRatio
	}
	return model.SidebarRatio
}

// Config snapshots everything config.json persists.
func (model Model) Config() Config {
	return Config{
		LastChat:         model.CurrentChat.Name,
		SidebarRatio:     model.SidebarRatio,
		SidebarCollapsed: model.SidebarCollapsed,
		Providers:        model.Providers,
		DefaultProvider:  model.DefaultProvider,
		DefaultModel:     model.DefaultModel,
	}
}

// SettingsState drives the settings modal. Draft is the provider catalogue
// being edited (applied to the live config only on SaveSettings); Selected
// indexes the provider the text fields show. Epoch keys the rebuild of the
// uncontrolled fields — bumped on open and on provider add/remove/select/
// template, NOT on keystrokes, so typing never reseeds the field under the
// cursor. Dropdown is whether the global default-model picker is open.
// EditGen counts key/URL keystrokes; only the settle timer carrying the
// latest generation may check the key with a /models fetch. Errors holds
// the last fetch outcome per provider name — no entry means no result yet,
// "" means the key checked out, anything else is the fetch error.
type SettingsState struct {
	Open            bool
	Epoch           int
	Draft           []Provider
	Selected        int
	DefaultProvider string
	DefaultModel    string
	Dropdown        bool
	EditGen         int
	Errors          map[string]string
}

// KeyStatus is the settings pane's verdict on one provider's API key,
// derived from the Errors map's tri-state entries.
type KeyStatus int

const (
	KeyMissing  KeyStatus = iota // no key entered
	KeyChecking                  // no /models result yet
	KeyOK                        // the last fetch succeeded
	KeyBad                       // the last fetch failed
)

// KeyStatus classifies the provider's API key by the last /models outcome.
// Keys are trimmed on entry, so an all-blank key never reaches the draft.
func (s SettingsState) KeyStatus(p Provider) KeyStatus {
	err, checked := s.Errors[p.Name]
	switch {
	case p.APIKey == "":
		return KeyMissing
	case !checked:
		return KeyChecking
	case err != "":
		return KeyBad
	}
	return KeyOK
}

// ProviderTemplates is the settings template bar: OpenAI-v1-compatible
// endpoints whose Name and BaseURL prefill the selected provider. OpenAI
// leads (an empty BaseURL is the OpenAI default).
var ProviderTemplates = []Provider{
	{Name: "OpenAI"},
	{Name: "xAI", BaseURL: "https://api.x.ai/v1"},
	{Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "Groq", BaseURL: "https://api.groq.com/openai/v1"},
}

// SelectedProvider returns the draft provider the fields edit.
func (s SettingsState) SelectedProvider() (Provider, bool) {
	if s.Selected < 0 || s.Selected >= len(s.Draft) {
		return Provider{}, false
	}
	return s.Draft[s.Selected], true
}

// ProviderNamed resolves a provider by name in the live catalogue.
func (model Model) ProviderNamed(name string) (Provider, bool) {
	for _, p := range model.Providers {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}

// EffectiveModel resolves the provider and model id prompts in the current
// chat use: the chat's override while it still names a live provider, else
// the global default, else the first configured provider with its first
// cached model. The bool is false when no provider is configured at all.
func (model Model) EffectiveModel() (Provider, string, bool) {
	if model.CurrentChat.Provider != "" {
		if p, ok := model.ProviderNamed(model.CurrentChat.Provider); ok {
			return p, model.CurrentChat.Model, true
		}
	}
	if p, ok := model.ProviderNamed(model.DefaultProvider); ok {
		return p, model.DefaultModel, true
	}
	if len(model.Providers) > 0 {
		p := model.Providers[0]
		id := ""
		if len(p.Models) > 0 {
			id = p.Models[0]
		}
		return p, id, true
	}
	return Provider{}, "", false
}

// StreamState is one in-flight exchange: the chat it belongs to (kept up
// to date across renames) and, while that chat is not current, the
// accumulated history it will persist. Provider/Model mirror the chat's
// override so the stream's saves preserve it in the history file. Status
// is the transient server-side tool indicator the view shows under the
// current chat ("Searching the web…").
type StreamState struct {
	Chat     string
	Provider string
	Model    string
	Status   string
	History  []Message
}

// RenameState drives the rename modal: Target is the chat filename being
// renamed ("" = modal closed); Epoch is bumped on every open and keys the
// rebuild of the modal's uncontrolled text field, so reopening — even for
// the same chat — reseeds it.
type RenameState struct {
	Target string
	Epoch  int
}

// PendingDelete remembers everything needed to undo a chat delete: the name,
// where in the list it sat, and whether it was the selected chat.
type PendingDelete struct {
	Name       string
	Index      int
	WasCurrent bool
	Gen        int
}

func (model Model) ConfigFile() string {
	return filepath.Join(model.DataDir, "config.json")
}

func (model Model) ChatDir() string {
	return filepath.Join(model.DataDir, "chats")
}

func (model Model) ChatFile(name string) string {
	return filepath.Join(model.DataDir, "chats", name)
}

// LogDir holds the wire logs: one JSONL file per launch day recording
// every raw Responses API event, for post-mortems of a bad exchange.
func (model Model) LogDir() string {
	return filepath.Join(model.DataDir, "logs")
}

// TrashDir holds deleted chats' history files while they are undoable.
func (model Model) TrashDir() string {
	return filepath.Join(model.DataDir, "chats", ".trash")
}

func (model Model) TrashFile(name string) string {
	return filepath.Join(model.DataDir, "chats", ".trash", name)
}

// TakenNames returns every chat name that must not be reused: listed chats
// plus everything still undoable in the trash (restoring must never
// collide).
func (model Model) TakenNames() ChatList {
	names := slices.Clone(model.ChatList)
	for _, pending := range model.Trash {
		names = append(names, pending.Name)
	}
	return names
}

// StreamFor returns the id of the in-flight completion for the named chat.
func (model Model) StreamFor(name string) (int, bool) {
	for id, s := range model.Streams {
		if s.Chat == name {
			return id, true
		}
	}
	return 0, false
}

// cloneStreams returns a copy of streams for reducer-safe mutation (models
// are values, but maps are shared references).
func cloneStreams(streams map[int]StreamState) map[int]StreamState {
	next := make(map[int]StreamState, len(streams)+1)
	maps.Copy(next, streams)
	return next
}

// FreshProviderName returns the first of Provider, Provider 2, Provider 3,
// … not taken in the draft.
func FreshProviderName(draft []Provider) string {
	taken := func(name string) bool {
		return slices.ContainsFunc(draft, func(p Provider) bool { return p.Name == name })
	}
	name := "Provider"
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("Provider %d", i)
	}
	return name
}

// FreshChatName returns the first of new.jsonl, new-2.jsonl, new-3.jsonl,
// … not taken by existing.
func FreshChatName(existing ChatList) string {
	name := "new.jsonl"
	for i := 2; slices.Contains(existing, name); i++ {
		name = fmt.Sprintf("new-%d.jsonl", i)
	}
	return name
}
