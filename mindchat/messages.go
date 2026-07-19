package main

import "time"

type LoadState struct{}

// Message roles. User and assistant rows round-trip through the history
// file and the wire; error rows are persisted notices of a failed exchange;
// status rows are transient (the "Searching the web…" indicator the view
// appends while a stream's server-side tool runs) and never persisted.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleError     = "error"
	RoleStatus    = "status"
)

// Citation is one source an assistant answer referenced — a url_citation
// annotation delivered by a server-side web search.
type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// Message is one history entry as the view renders it. The lowercase JSON
// tags match the wire shape of the pre-JSONL history files, so legacy
// arrays decode into it directly.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Citations []Citation `json:"citations,omitempty"`
}

// Provider is one OpenAI-compatible API endpoint the app can talk to,
// spoken to over the Responses API (…/responses under BaseURL). An empty
// BaseURL means the OpenAI default; xAI, Groq and OpenRouter serve the same
// dialect under their own bases. Models caches the ids of the last
// successful /models fetch so pickers have entries while offline.
// WebSearch attaches the server-side web_search tool to every request —
// supported by xAI and OpenAI; leave it off for providers that reject
// unknown tools.
type Provider struct {
	Name      string
	BaseURL   string
	APIKey    string
	WebSearch bool
	Models    []string
}

type Config struct {
	LastChat string
	// SidebarRatio is the split-pane position (0 = default); Collapsed
	// remembers the [|] toggle across launches.
	SidebarRatio     float32
	SidebarCollapsed bool
	// Providers is the API endpoint catalogue the settings modal edits;
	// DefaultProvider/DefaultModel name the model new prompts use unless
	// the chat carries its own override.
	Providers       []Provider
	DefaultProvider string
	DefaultModel    string
}

// ToggleSidebar collapses the sidebar to an icon rail or restores it.
type ToggleSidebar struct{}

// SetSidebarRatio follows the split-pane divider drag; dragging below the
// rail threshold collapses, dragging out restores.
type SetSidebarRatio struct {
	Ratio float32
}

type Prompt struct {
	Content string
}

type ChatList []string

type SelectChat struct {
	Name string
}

type DeleteChat struct {
	Name string
}

// UndoDelete reverses the pending chat delete while its undo window is open.
type UndoDelete struct{}

// ConfirmDelete closes a delete's undo window; it only takes effect while
// Gen matches the pending delete's generation.
type ConfirmDelete struct {
	Gen int
}

// NewChat starts a fresh, empty chat and selects it.
type NewChat struct{}

// OpenRename opens the rename modal for the named chat.
type OpenRename struct {
	Name string
}

// CloseRename dismisses the rename modal without renaming.
type CloseRename struct{}

// RenameChat renames the modal's target chat to the given name (extension
// optional). The reducer validates; an invalid name leaves the modal open.
type RenameChat struct {
	To string
}

// OpenSettings opens the settings modal seeded with a draft copy of the
// live provider configuration, and kicks off a /models fetch per keyed
// provider so the model lists are fresh.
type OpenSettings struct{}

// CloseSettings dismisses the settings modal, discarding the draft.
type CloseSettings struct{}

// SaveSettings applies the draft to the live configuration, persists it,
// and closes the modal.
type SaveSettings struct{}

// AddProvider appends a blank provider to the draft and selects it.
type AddProvider struct{}

// RemoveProvider deletes the draft's selected provider.
type RemoveProvider struct{}

// SelectProvider makes the indexed draft provider the one the fields edit.
type SelectProvider struct {
	Index int
}

// ProviderField names which of the selected provider's fields an
// EditProvider keystroke applies to.
type ProviderField int

const (
	FieldName ProviderField = iota
	FieldBaseURL
	FieldAPIKey
)

// EditProvider mirrors one settings text field into the draft's selected
// provider on every keystroke (the fields are uncontrolled; the draft is
// the source of truth on Save).
type EditProvider struct {
	Field ProviderField
	Text  string
}

// ToggleWebSearch flips the draft's selected provider's web_search tool.
type ToggleWebSearch struct{}

// ApplyTemplate prefills the draft's selected provider from the template
// bar entry (Name + BaseURL; the API key stays as typed).
type ApplyTemplate struct {
	Index int
}

// OpenDefaultModelMenu / CloseDefaultModelMenu drive the settings modal's
// default-model dropdown.
type OpenDefaultModelMenu struct{}
type CloseDefaultModelMenu struct{}

// SetDefaultModel picks the draft's default (provider, model) pair.
type SetDefaultModel struct {
	Provider string
	Model    string
}

// RefreshModels re-fetches /models for the draft's selected provider.
type RefreshModels struct{}

// ProviderEditSettled fires a debounce interval after a key/URL keystroke:
// when Gen still matches the draft's EditGen (no later keystroke), the
// named provider's key is checked with a /models fetch.
type ProviderEditSettled struct {
	Gen      int
	Provider string
}

// ModelsFetched reports a /models call for the named provider: the model
// ids on success, or Err. It updates the live cache (persisted) and, while
// the settings modal is open, the draft and its error line.
type ModelsFetched struct {
	Provider string
	Models   []string
	Err      string
}

// OpenModelMenu / CloseModelMenu drive the chat header's model picker.
type OpenModelMenu struct{}
type CloseModelMenu struct{}

// SetChatModel records a per-chat model override on the current chat (and
// its history file). Empty Provider and Model revert the chat to the
// global default.
type SetChatModel struct {
	Provider string
	Model    string
}

// Chat is the conversation being displayed. Provider/Model, when set,
// override the global default for prompts sent from this chat; they
// persist in the chat's history file. Loaded flags that History reflects
// the file (or a live stream) — guarding writes against clobbering a chat
// whose disk load is still in flight.
type Chat struct {
	Name     string
	Provider string
	Model    string
	Loaded   bool
	History  []Message
}

// ChatEvent is one line of a chat's .jsonl history file — the append-only
// conversation log. Type keys which payload group is meaningful: "meta"
// carries the chat's model override from that point on, "user"/"assistant"
// carry a message (assistant may add citations), "error" records what
// ended an exchange abnormally. Unknown types are skipped on replay, so
// the format can grow without breaking older builds.
type ChatEvent struct {
	Time      time.Time  `json:"time"`
	Type      string     `json:"type"`
	Provider  string     `json:"provider,omitempty"`
	Model     string     `json:"model,omitempty"`
	Text      string     `json:"text,omitempty"`
	Citations []Citation `json:"citations,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// ChatFile is a chat history replayed into memory: the model override the
// events left in effect plus the message rows. ParseChatFile builds it
// from any of the three on-disk formats (JSONL events, the legacy wrapped
// object, the legacy bare history array).
type ChatFile struct {
	Provider string
	Model    string
	History  []Message
}

// Stream messages tag Responses API events with the stream that produced
// them, so the reducer can route them to the chat that asked — even after
// the user switches away, renames the chat, or deletes it.

// AssistantDelta appends streamed text to the stream's open assistant row
// (opening one when the exchange has none yet).
type AssistantDelta struct {
	Stream int
	Text   string
}

// ToolStatus reports a server-side tool's progress ("Searching the web…");
// empty Status clears the indicator. Transient — never persisted.
type ToolStatus struct {
	Stream int
	Status string
}

// CitationAdded attaches a web-search source to the stream's assistant row.
type CitationAdded struct {
	Stream int
	URL    string
	Title  string
}

// StreamCompleted is the response.completed event: the exchange succeeded
// and the assistant row is final — persist it to the stream's chat file.
type StreamCompleted struct {
	Stream int
}

// StreamFailed reports a failed exchange: an error event, a failed or
// incomplete response, or a dead connection. The reducer persists any
// partial answer plus the error, and shows the error in the chat.
type StreamFailed struct {
	Stream int
	Err    string
}

// StreamDone marks the transport closed for ANY reason. After a normal
// StreamCompleted (or a StreamFailed) it is a no-op; a stream still
// tracked here ended silently and is treated as failed.
type StreamDone struct {
	Stream int
}

// HistLoaded tags a history read with the chat it belongs to, so a slow
// load can never overwrite a different (or streaming) conversation.
// Provider/Model carry the chat's persisted model override.
type HistLoaded struct {
	Chat     string
	Provider string
	Model    string
	History  []Message
}
