package main

import (
	"testing"
)

// The Update reducer is a pure function from (Model, message) to Model — the
// commands it returns are exercised by the runtime, but the state transitions
// are testable directly (cf. todos/redux_test.go).

func testModel() Model {
	return Model{
		DataDir:   "/tmp/mindchat-test",
		AuthToken: "test-token",
		CurrentChat: Chat{
			Name: "alpha.jsonl",
			History: []Message{
				{Role: RoleUser, Content: "hi"},
			},
		},
		ChatList: ChatList{"alpha.jsonl", "beta.jsonl"},
	}
}

func TestUpdateSelectChatSwitchesAndClearsHistory(t *testing.T) {
	next, _ := Update(testModel(), SelectChat{Name: "beta.jsonl"})
	if next.CurrentChat.Name != "beta.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "beta.jsonl")
	}
	if next.CurrentChat.History != nil {
		t.Fatalf("History = %v, want nil (cleared while the selected chat loads)", next.CurrentChat.History)
	}
}

func TestUpdateSelectChatSameChatIsNoOp(t *testing.T) {
	model := testModel()
	next, _ := Update(model, SelectChat{Name: model.CurrentChat.Name})
	if len(next.CurrentChat.History) != len(model.CurrentChat.History) {
		t.Fatalf("re-selecting the current chat must not clear its history")
	}
}

func TestUpdatePromptAppendsUserMessage(t *testing.T) {
	next, _ := Update(testModel(), Prompt{Content: "hello"})
	hist := next.CurrentChat.History
	if len(hist) != 2 {
		t.Fatalf("len(History) = %d, want 2", len(hist))
	}
	last := hist[len(hist)-1]
	if last.Role != RoleUser || last.Content != "hello" {
		t.Fatalf("last message = %+v, want user %q", last, "hello")
	}
}

func TestUpdateEmptyPromptIsNoOp(t *testing.T) {
	model := testModel()
	next, _ := Update(model, Prompt{Content: ""})
	if len(next.CurrentChat.History) != len(model.CurrentChat.History) {
		t.Fatalf("empty prompt must not append to history")
	}
}

func TestUpdateStreamDeltasOpenThenAppend(t *testing.T) {
	model, _ := Update(testModel(), Prompt{Content: "hi again"}) // stream 1

	// The exchange's first delta opens the assistant row...
	opened, _ := Update(model, AssistantDelta{Stream: 1, Text: "Hel"})
	hist := opened.CurrentChat.History
	if len(hist) != 3 || hist[2].Role != RoleAssistant || hist[2].Content != "Hel" {
		t.Fatalf("after first delta: history = %+v", hist)
	}

	// ...and later ones extend it.
	extended, _ := Update(opened, AssistantDelta{Stream: 1, Text: "lo"})
	hist = extended.CurrentChat.History
	if hist[2].Content != "Hello" {
		t.Fatalf("assistant content = %q, want %q", hist[2].Content, "Hello")
	}

	// Completing unregisters the stream.
	done, _ := Update(extended, StreamCompleted{Stream: 1})
	if len(done.Streams) != 0 {
		t.Fatalf("Streams = %+v, want empty after completion", done.Streams)
	}
}

func TestUpdateStreamAfterSwitchBuffersToOwningChat(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "tell me a story"}) // alpha, stream 1
	m, _ = Update(m, SelectChat{Name: "beta.jsonl"})
	m, _ = Update(m, HistLoaded{Chat: "beta.jsonl", History: []Message{
		{Role: RoleUser, Content: "beta's own question"},
	}})

	// alpha's deltas keep arriving; they must not touch beta's view.
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: "Once upon"})
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: " a time"})

	if len(m.CurrentChat.History) != 1 || m.CurrentChat.History[0].Content != "beta's own question" {
		t.Fatalf("beta's history was touched by alpha's stream: %+v", m.CurrentChat.History)
	}
	s := m.Streams[1]
	if s.Chat != "alpha.jsonl" || len(s.History) != 3 || s.History[2].Content != "Once upon a time" {
		t.Fatalf("alpha's buffer = %+v, want its prompt history plus the reply", s)
	}
}

func TestUpdateSwitchBackAdoptsLiveStream(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "story?"}) // alpha, stream 1
	m, _ = Update(m, SelectChat{Name: "beta.jsonl"})
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: "Once"})

	// Switching back must show the live buffer, not start a disk load.
	m, cmd := Update(m, SelectChat{Name: "alpha.jsonl"})
	_ = cmd
	hist := m.CurrentChat.History
	if len(hist) != 3 || hist[2].Content != "Once" {
		t.Fatalf("adopted history = %+v, want the streaming buffer", hist)
	}
	// ...and later deltas apply to the visible history again.
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: " upon"})
	if m.CurrentChat.History[2].Content != "Once upon" {
		t.Fatalf("delta after switch-back = %q", m.CurrentChat.History[2].Content)
	}
	// A stale disk load for alpha must not clobber the live stream.
	m, _ = Update(m, HistLoaded{Chat: "alpha.jsonl", History: nil})
	if m.CurrentChat.History[2].Content != "Once upon" {
		t.Fatalf("stale HistLoaded clobbered a streaming chat")
	}
}

func TestUpdateStreamDeltaAfterDeleteIsDropped(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "story?"}) // alpha, stream 1
	m, _ = Update(m, DeleteChat{Name: "alpha.jsonl"})
	if len(m.Streams) != 0 {
		t.Fatalf("Streams = %+v, want dropped on delete", m.Streams)
	}
	next, _ := Update(m, AssistantDelta{Stream: 1, Text: "Once"})
	if n := len(next.CurrentChat.History); n != 0 && next.CurrentChat.History[n-1].Content == "Once" {
		t.Fatalf("untracked delta landed in %q", next.CurrentChat.Name)
	}
}

func TestUpdateRenameRetargetsStream(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "story?"}) // alpha, stream 1
	m, _ = Update(m, OpenRename{Name: "alpha.jsonl"})
	m, _ = Update(m, RenameChat{To: "ideas"})
	if s := m.Streams[1]; s.Chat != "ideas.jsonl" {
		t.Fatalf("stream chat = %q, want ideas.jsonl after rename", s.Chat)
	}
	// Deltas keep applying to the (renamed, still current) chat.
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: "Once"})
	hist := m.CurrentChat.History
	if len(hist) != 3 || hist[2].Content != "Once" {
		t.Fatalf("history after rename mid-stream = %+v", hist)
	}
}

func TestUpdateStreamDoneTreatsSilentEndAsFailure(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "hi there"}) // stream 1
	if len(m.Streams) != 1 {
		t.Fatalf("Streams = %+v, want the prompt's stream registered", m.Streams)
	}
	// The transport dies without completed/failed events: StreamDone must
	// unregister the stream AND surface an error row — a silent ending is
	// exactly the bug this path guards against.
	done, _ := Update(m, StreamDone{Stream: 1})
	if len(done.Streams) != 0 {
		t.Fatalf("Streams = %+v, want cleaned up after StreamDone", done.Streams)
	}
	hist := done.CurrentChat.History
	if len(hist) != 3 || hist[2].Role != RoleError {
		t.Fatalf("History = %+v, want the prompt kept plus an error row", hist)
	}
	// A StreamDone after normal StreamCompleted cleanup is a no-op.
	again, _ := Update(done, StreamDone{Stream: 1})
	if len(again.CurrentChat.History) != 3 {
		t.Fatalf("repeat StreamDone must be a no-op")
	}
}

func TestUpdateStreamFailedShowsErrorRow(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "search please"}) // stream 1
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: "I'll search"})
	failed, _ := Update(m, StreamFailed{Stream: 1, Err: "HTTP 410: Gone"})
	hist := failed.CurrentChat.History
	if len(hist) != 4 || hist[3].Role != RoleError || hist[3].Content != "HTTP 410: Gone" {
		t.Fatalf("History = %+v, want the partial answer plus the error row", hist)
	}
	if len(failed.Streams) != 0 {
		t.Fatalf("Streams = %+v, want unregistered after failure", failed.Streams)
	}
}

func TestUpdateToolStatusTracksStream(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "look this up"}) // stream 1
	m, _ = Update(m, ToolStatus{Stream: 1, Status: "Searching the web…"})
	if m.Streams[1].Status != "Searching the web…" {
		t.Fatalf("Status = %q, want the search indicator", m.Streams[1].Status)
	}
	m, _ = Update(m, ToolStatus{Stream: 1, Status: ""})
	if m.Streams[1].Status != "" {
		t.Fatalf("Status = %q, want cleared when the tool finishes", m.Streams[1].Status)
	}
	// An untracked stream's status is dropped.
	next, _ := Update(m, ToolStatus{Stream: 9, Status: "Searching…"})
	if _, ok := next.Streams[9]; ok {
		t.Fatalf("untracked ToolStatus registered a stream")
	}
}

func TestUpdateCitationsAttachAndDedupe(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "look this up"}) // stream 1
	// A citation may arrive before any text delta: it opens the row.
	m, _ = Update(m, CitationAdded{Stream: 1, URL: "https://x.ai", Title: "xAI"})
	m, _ = Update(m, AssistantDelta{Stream: 1, Text: "Answer."})
	m, _ = Update(m, CitationAdded{Stream: 1, URL: "https://x.ai", Title: "xAI"}) // duplicate
	m, _ = Update(m, CitationAdded{Stream: 1, URL: "https://openai.com"})
	hist := m.CurrentChat.History
	last := hist[len(hist)-1]
	if last.Role != RoleAssistant || last.Content != "Answer." {
		t.Fatalf("last row = %+v, want the assistant answer", last)
	}
	if len(last.Citations) != 2 || last.Citations[0].URL != "https://x.ai" || last.Citations[1].URL != "https://openai.com" {
		t.Fatalf("Citations = %+v, want two deduped sources", last.Citations)
	}
}

func TestUpdateStaleHistLoadIsIgnored(t *testing.T) {
	m := testModel() // current: alpha
	next, _ := Update(m, HistLoaded{Chat: "beta.jsonl", History: []Message{
		{Role: RoleUser, Content: "beta content"},
	}})
	if len(next.CurrentChat.History) != 1 || next.CurrentChat.History[0].Content != "hi" {
		t.Fatalf("a load tagged for beta was applied to alpha: %+v", next.CurrentChat.History)
	}
}

func TestUpdateDeltaOnClearedHistoryOpensRow(t *testing.T) {
	m, _ := Update(testModel(), Prompt{Content: "hi"}) // stream 1
	m.CurrentChat.History = nil                        // simulate the cleared-view window
	next, _ := Update(m, AssistantDelta{Stream: 1, Text: "orphan"})
	hist := next.CurrentChat.History
	if len(hist) != 1 || hist[0].Role != RoleAssistant || hist[0].Content != "orphan" {
		t.Fatalf("delta on cleared history = %+v, want a fresh assistant row (no text lost)", hist)
	}
}

func TestUpdateConfigRemapsLegacyLastChat(t *testing.T) {
	next, _ := Update(Model{}, Config{LastChat: "monoid.json"})
	if next.CurrentChat.Name != "monoid.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want the migrated .jsonl name", next.CurrentChat.Name)
	}
}

func TestUpdateDeleteChatKeepsCurrentWhenOtherDeleted(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "beta.jsonl"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "alpha.jsonl" {
		t.Fatalf("ChatList = %v, want [alpha.jsonl]", next.ChatList)
	}
	if next.CurrentChat.Name != "alpha.jsonl" || len(next.CurrentChat.History) != 1 {
		t.Fatalf("current chat changed: %+v", next.CurrentChat)
	}
}

func TestUpdateDeleteCurrentChatSelectsFirstRemaining(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "alpha.jsonl"})
	if next.CurrentChat.Name != "beta.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "beta.jsonl")
	}
	if next.CurrentChat.History != nil {
		t.Fatalf("History = %v, want nil (cleared while the fallback chat loads)", next.CurrentChat.History)
	}
	if len(next.ChatList) != 1 || next.ChatList[0] != "beta.jsonl" {
		t.Fatalf("ChatList = %v, want [beta.jsonl]", next.ChatList)
	}
}

func TestUpdateDeleteLastChatClearsCurrent(t *testing.T) {
	model := testModel()
	model.ChatList = ChatList{"alpha.jsonl"}
	next, _ := Update(model, DeleteChat{Name: "alpha.jsonl"})
	if next.CurrentChat.Name != "" || next.CurrentChat.History != nil {
		t.Fatalf("current chat not cleared: %+v", next.CurrentChat)
	}
	if len(next.ChatList) != 0 {
		t.Fatalf("ChatList = %v, want empty", next.ChatList)
	}
}

func TestUpdatePromptWithNoChatStartsFreshOne(t *testing.T) {
	model := testModel()
	model.CurrentChat = Chat{}
	model.ChatList = nil
	next, _ := Update(model, Prompt{Content: "hello"})
	if next.CurrentChat.Name != "new.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "new.jsonl")
	}
	if len(next.ChatList) != 1 || next.ChatList[0] != "new.jsonl" {
		t.Fatalf("ChatList = %v, want [new.jsonl]", next.ChatList)
	}
	if len(next.CurrentChat.History) != 1 {
		t.Fatalf("History = %v, want the prompt appended", next.CurrentChat.History)
	}
}

func TestUpdateDeleteChatOpensUndoWindow(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "alpha.jsonl"})
	want := PendingDelete{Name: "alpha.jsonl", Index: 0, WasCurrent: true, Gen: 1}
	if next.Pending != want {
		t.Fatalf("Pending = %+v, want %+v", next.Pending, want)
	}
	if len(next.Trash) != 1 || next.Trash[0] != want {
		t.Fatalf("Trash = %+v, want the delete pushed onto the stack", next.Trash)
	}
}

func TestUpdateUndoAfterBarHiddenStillRestores(t *testing.T) {
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.jsonl"})
	hidden, _ := Update(deleted, ConfirmDelete{Gen: 1}) // bar timer fired
	if hidden.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want bar hidden", hidden.Pending)
	}
	if len(hidden.Trash) != 1 {
		t.Fatalf("Trash = %+v, want delete still undoable after the bar hides", hidden.Trash)
	}
	restored, _ := Update(hidden, UndoDelete{})
	if len(restored.ChatList) != 2 || restored.ChatList[0] != "alpha.jsonl" {
		t.Fatalf("ChatList = %v, want alpha.jsonl restored", restored.ChatList)
	}
	if len(restored.Trash) != 0 {
		t.Fatalf("Trash = %+v, want emptied", restored.Trash)
	}
}

func TestUpdateUndoPopsStackMostRecentFirst(t *testing.T) {
	m := testModel()
	m, _ = Update(m, DeleteChat{Name: "beta.jsonl"})
	m, _ = Update(m, DeleteChat{Name: "alpha.jsonl"})
	if len(m.Trash) != 2 || len(m.ChatList) != 0 {
		t.Fatalf("after two deletes: Trash=%+v ChatList=%v", m.Trash, m.ChatList)
	}
	m, _ = Update(m, UndoDelete{})
	if len(m.ChatList) != 1 || m.ChatList[0] != "alpha.jsonl" {
		t.Fatalf("first undo restored %v, want the most recent (alpha.jsonl)", m.ChatList)
	}
	m, _ = Update(m, UndoDelete{})
	if len(m.ChatList) != 2 {
		t.Fatalf("second undo: ChatList = %v, want both back", m.ChatList)
	}
	m, _ = Update(m, UndoDelete{})
	if len(m.ChatList) != 2 {
		t.Fatalf("undo on an empty stack must be a no-op")
	}
}

func TestUpdateUndoRestoresChatAtItsIndex(t *testing.T) {
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.jsonl"})
	restored, _ := Update(deleted, UndoDelete{})
	if restored.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want cleared", restored.Pending)
	}
	if len(restored.ChatList) != 2 || restored.ChatList[0] != "alpha.jsonl" || restored.ChatList[1] != "beta.jsonl" {
		t.Fatalf("ChatList = %v, want [alpha.jsonl beta.jsonl]", restored.ChatList)
	}
	if restored.CurrentChat.Name != "alpha.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want the restored chat re-selected", restored.CurrentChat.Name)
	}
}

func TestUpdateStaleConfirmIsIgnored(t *testing.T) {
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.jsonl"}) // gen 1
	restored, _ := Update(deleted, UndoDelete{})
	redeleted, _ := Update(restored, DeleteChat{Name: "beta.jsonl"}) // gen 2

	// Generation 1's timer fires late: it must not finalise generation 2.
	next, _ := Update(redeleted, ConfirmDelete{Gen: 1})
	if next.Pending.Name != "beta.jsonl" {
		t.Fatalf("stale ConfirmDelete finalised the wrong delete: Pending = %+v", next.Pending)
	}
	// Generation 2's own timer does close it.
	next, _ = Update(next, ConfirmDelete{Gen: 2})
	if next.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want cleared", next.Pending)
	}
}

func TestUpdateSecondDeleteReplacesBarKeepsBothUndoable(t *testing.T) {
	first, _ := Update(testModel(), DeleteChat{Name: "beta.jsonl"})
	second, _ := Update(first, DeleteChat{Name: "alpha.jsonl"})
	if second.Pending.Name != "alpha.jsonl" || second.Pending.Gen != 2 {
		t.Fatalf("Pending = %+v, want the bar showing alpha.jsonl at gen 2", second.Pending)
	}
	if len(second.ChatList) != 0 {
		t.Fatalf("ChatList = %v, want empty", second.ChatList)
	}
	if len(second.Trash) != 2 {
		t.Fatalf("Trash = %+v, want both deletes undoable", second.Trash)
	}
}

func TestUpdateNewChatSelectsFreshName(t *testing.T) {
	model := testModel()
	model.ChatList = ChatList{"new.jsonl"}
	next, _ := Update(model, NewChat{})
	if next.CurrentChat.Name != "new-2.jsonl" || next.CurrentChat.History != nil {
		t.Fatalf("CurrentChat = %+v, want empty new-2.jsonl", next.CurrentChat)
	}
	if len(next.ChatList) != 2 || next.ChatList[1] != "new-2.jsonl" {
		t.Fatalf("ChatList = %v, want [new.jsonl new-2.jsonl]", next.ChatList)
	}
}

func TestUpdateOpenRenameTargetsChatAndBumpsEpoch(t *testing.T) {
	next, _ := Update(testModel(), OpenRename{Name: "beta.jsonl"})
	if next.Rename.Target != "beta.jsonl" || next.Rename.Epoch != 1 {
		t.Fatalf("Rename = %+v, want beta.jsonl at epoch 1", next.Rename)
	}
	reopened, _ := Update(next, OpenRename{Name: "beta.jsonl"})
	if reopened.Rename.Epoch != 2 {
		t.Fatalf("Epoch = %d, want 2 (bumped on every open)", reopened.Rename.Epoch)
	}
	unknown, _ := Update(testModel(), OpenRename{Name: "nope.jsonl"})
	if unknown.Rename.Target != "" {
		t.Fatalf("Rename = %+v, want closed for unknown chat", unknown.Rename)
	}
}

func TestUpdateRenameChatRenamesCurrentInPlace(t *testing.T) {
	opened, _ := Update(testModel(), OpenRename{Name: "alpha.jsonl"})
	next, _ := Update(opened, RenameChat{To: "ideas"}) // extension added
	if next.ChatList[0] != "ideas.jsonl" || next.ChatList[1] != "beta.jsonl" {
		t.Fatalf("ChatList = %v, want [ideas.jsonl beta.jsonl] (index kept)", next.ChatList)
	}
	if next.CurrentChat.Name != "ideas.jsonl" {
		t.Fatalf("CurrentChat.Name = %q, want ideas.jsonl", next.CurrentChat.Name)
	}
	if len(next.CurrentChat.History) != 1 {
		t.Fatalf("rename must not touch the loaded history")
	}
	if next.Rename.Target != "" {
		t.Fatalf("modal still open: %+v", next.Rename)
	}
}

func TestUpdateRenameChatConvertsTypedLegacyExtension(t *testing.T) {
	opened, _ := Update(testModel(), OpenRename{Name: "alpha.jsonl"})
	next, _ := Update(opened, RenameChat{To: "notes.json"})
	if next.ChatList[0] != "notes.jsonl" {
		t.Fatalf("ChatList = %v, want a typed .json to follow the store to .jsonl", next.ChatList)
	}
}

func TestUpdateRenameChatRejectsInvalidNames(t *testing.T) {
	opened, _ := Update(testModel(), OpenRename{Name: "alpha.jsonl"})

	dup, _ := Update(opened, RenameChat{To: "beta"})
	if dup.ChatList[0] != "alpha.jsonl" || dup.Rename.Target == "" {
		t.Fatalf("duplicate name must be rejected with the modal kept open: %+v", dup.Rename)
	}

	sep, _ := Update(opened, RenameChat{To: "a/b"})
	if sep.ChatList[0] != "alpha.jsonl" || sep.Rename.Target == "" {
		t.Fatalf("path separators must be rejected with the modal kept open: %+v", sep.Rename)
	}

	empty, _ := Update(opened, RenameChat{To: "  "})
	if empty.ChatList[0] != "alpha.jsonl" || empty.Rename.Target != "" {
		t.Fatalf("empty submit must close without renaming: %+v", empty.Rename)
	}

	same, _ := Update(opened, RenameChat{To: "alpha"})
	if same.ChatList[0] != "alpha.jsonl" || same.Rename.Target != "" {
		t.Fatalf("unchanged name must close without renaming: %+v", same.Rename)
	}
}

func TestUpdateToggleSidebarFlipsAndPersists(t *testing.T) {
	collapsed, _ := Update(testModel(), ToggleSidebar{})
	if !collapsed.SidebarCollapsed {
		t.Fatalf("SidebarCollapsed = false, want true after toggle")
	}
	if collapsed.EffectiveRatio() != CollapsedRatio {
		t.Fatalf("EffectiveRatio = %v, want the rail ratio", collapsed.EffectiveRatio())
	}
	restored, _ := Update(collapsed, ToggleSidebar{})
	if restored.SidebarCollapsed {
		t.Fatalf("second toggle must restore")
	}
	if restored.EffectiveRatio() != DefaultSidebarRatio {
		t.Fatalf("EffectiveRatio = %v, want the default", restored.EffectiveRatio())
	}
}

func TestUpdateSetSidebarRatioDragAndCollapse(t *testing.T) {
	widened, _ := Update(testModel(), SetSidebarRatio{Ratio: 0.35})
	if widened.SidebarRatio != 0.35 || widened.SidebarCollapsed {
		t.Fatalf("drag to 0.35: ratio=%v collapsed=%v", widened.SidebarRatio, widened.SidebarCollapsed)
	}
	// Dragging under the rail threshold collapses but keeps the stored
	// width, so the toggle restores it.
	collapsed, _ := Update(widened, SetSidebarRatio{Ratio: 0.05})
	if !collapsed.SidebarCollapsed || collapsed.SidebarRatio != 0.35 {
		t.Fatalf("drag to rail: ratio=%v collapsed=%v", collapsed.SidebarRatio, collapsed.SidebarCollapsed)
	}
	restored, _ := Update(collapsed, ToggleSidebar{})
	if restored.EffectiveRatio() != 0.35 {
		t.Fatalf("restore = %v, want the pre-collapse 0.35", restored.EffectiveRatio())
	}
}

func TestUpdateConfigAdoptsSidebarState(t *testing.T) {
	next, _ := Update(testModel(), Config{LastChat: "alpha.jsonl", SidebarRatio: 0.3, SidebarCollapsed: true})
	if next.SidebarRatio != 0.3 || !next.SidebarCollapsed {
		t.Fatalf("sidebar state not adopted from config: %+v", next)
	}
}

func TestUpdateChatListReplacesList(t *testing.T) {
	next, _ := Update(testModel(), ChatList{"gamma.jsonl"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "gamma.jsonl" {
		t.Fatalf("ChatList = %v, want [gamma.jsonl]", next.ChatList)
	}
}
