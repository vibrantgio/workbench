package main

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// The Update reducer is a pure function from (Model, message) to Model — the
// commands it returns are exercised by the runtime, but the state transitions
// are testable directly (cf. todos/redux_test.go).

func testModel() Model {
	return Model{
		DataDir:   "/tmp/mindchat-test",
		AuthToken: "test-token",
		CurrentChat: Chat{
			Name: "alpha.json",
			History: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "hi"},
			},
		},
		ChatList: ChatList{"alpha.json", "beta.json"},
	}
}

func TestUpdateSelectChatSwitchesAndClearsHistory(t *testing.T) {
	next, _ := Update(testModel(), SelectChat{Name: "beta.json"})
	if next.CurrentChat.Name != "beta.json" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "beta.json")
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
	if last.Role != openai.ChatMessageRoleUser || last.Content != "hello" {
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
	model := testModel()

	// A delta carrying a role opens a new history entry...
	opened, _ := Update(model, openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{Role: openai.ChatMessageRoleAssistant, Content: "Hel"},
		}},
	})
	hist := opened.CurrentChat.History
	if len(hist) != 2 || hist[1].Role != openai.ChatMessageRoleAssistant || hist[1].Content != "Hel" {
		t.Fatalf("after role delta: history = %+v", hist)
	}

	// ...and role-less deltas extend it.
	extended, _ := Update(opened, openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{Content: "lo"},
		}},
	})
	hist = extended.CurrentChat.History
	if hist[1].Content != "Hello" {
		t.Fatalf("assistant content = %q, want %q", hist[1].Content, "Hello")
	}
}

func TestUpdateDeleteChatKeepsCurrentWhenOtherDeleted(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "beta.json"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "alpha.json" {
		t.Fatalf("ChatList = %v, want [alpha.json]", next.ChatList)
	}
	if next.CurrentChat.Name != "alpha.json" || len(next.CurrentChat.History) != 1 {
		t.Fatalf("current chat changed: %+v", next.CurrentChat)
	}
}

func TestUpdateDeleteCurrentChatSelectsFirstRemaining(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "alpha.json"})
	if next.CurrentChat.Name != "beta.json" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "beta.json")
	}
	if next.CurrentChat.History != nil {
		t.Fatalf("History = %v, want nil (cleared while the fallback chat loads)", next.CurrentChat.History)
	}
	if len(next.ChatList) != 1 || next.ChatList[0] != "beta.json" {
		t.Fatalf("ChatList = %v, want [beta.json]", next.ChatList)
	}
}

func TestUpdateDeleteLastChatClearsCurrent(t *testing.T) {
	model := testModel()
	model.ChatList = ChatList{"alpha.json"}
	next, _ := Update(model, DeleteChat{Name: "alpha.json"})
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
	if next.CurrentChat.Name != "new.json" {
		t.Fatalf("CurrentChat.Name = %q, want %q", next.CurrentChat.Name, "new.json")
	}
	if len(next.ChatList) != 1 || next.ChatList[0] != "new.json" {
		t.Fatalf("ChatList = %v, want [new.json]", next.ChatList)
	}
	if len(next.CurrentChat.History) != 1 {
		t.Fatalf("History = %v, want the prompt appended", next.CurrentChat.History)
	}
}

func TestUpdateDeleteChatOpensUndoWindow(t *testing.T) {
	next, _ := Update(testModel(), DeleteChat{Name: "alpha.json"})
	want := PendingDelete{Name: "alpha.json", Index: 0, WasCurrent: true, Gen: 1}
	if next.Pending != want {
		t.Fatalf("Pending = %+v, want %+v", next.Pending, want)
	}
	if len(next.Trash) != 1 || next.Trash[0] != want {
		t.Fatalf("Trash = %+v, want the delete pushed onto the stack", next.Trash)
	}
}

func TestUpdateUndoAfterBarHiddenStillRestores(t *testing.T) {
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.json"})
	hidden, _ := Update(deleted, ConfirmDelete{Gen: 1}) // bar timer fired
	if hidden.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want bar hidden", hidden.Pending)
	}
	if len(hidden.Trash) != 1 {
		t.Fatalf("Trash = %+v, want delete still undoable after the bar hides", hidden.Trash)
	}
	restored, _ := Update(hidden, UndoDelete{})
	if len(restored.ChatList) != 2 || restored.ChatList[0] != "alpha.json" {
		t.Fatalf("ChatList = %v, want alpha.json restored", restored.ChatList)
	}
	if len(restored.Trash) != 0 {
		t.Fatalf("Trash = %+v, want emptied", restored.Trash)
	}
}

func TestUpdateUndoPopsStackMostRecentFirst(t *testing.T) {
	m := testModel()
	m, _ = Update(m, DeleteChat{Name: "beta.json"})
	m, _ = Update(m, DeleteChat{Name: "alpha.json"})
	if len(m.Trash) != 2 || len(m.ChatList) != 0 {
		t.Fatalf("after two deletes: Trash=%+v ChatList=%v", m.Trash, m.ChatList)
	}
	m, _ = Update(m, UndoDelete{})
	if len(m.ChatList) != 1 || m.ChatList[0] != "alpha.json" {
		t.Fatalf("first undo restored %v, want the most recent (alpha.json)", m.ChatList)
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
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.json"})
	restored, _ := Update(deleted, UndoDelete{})
	if restored.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want cleared", restored.Pending)
	}
	if len(restored.ChatList) != 2 || restored.ChatList[0] != "alpha.json" || restored.ChatList[1] != "beta.json" {
		t.Fatalf("ChatList = %v, want [alpha.json beta.json]", restored.ChatList)
	}
	if restored.CurrentChat.Name != "alpha.json" {
		t.Fatalf("CurrentChat.Name = %q, want the restored chat re-selected", restored.CurrentChat.Name)
	}
}

func TestUpdateStaleConfirmIsIgnored(t *testing.T) {
	deleted, _ := Update(testModel(), DeleteChat{Name: "alpha.json"}) // gen 1
	restored, _ := Update(deleted, UndoDelete{})
	redeleted, _ := Update(restored, DeleteChat{Name: "beta.json"}) // gen 2

	// Generation 1's timer fires late: it must not finalise generation 2.
	next, _ := Update(redeleted, ConfirmDelete{Gen: 1})
	if next.Pending.Name != "beta.json" {
		t.Fatalf("stale ConfirmDelete finalised the wrong delete: Pending = %+v", next.Pending)
	}
	// Generation 2's own timer does close it.
	next, _ = Update(next, ConfirmDelete{Gen: 2})
	if next.Pending.Name != "" {
		t.Fatalf("Pending = %+v, want cleared", next.Pending)
	}
}

func TestUpdateSecondDeleteReplacesBarKeepsBothUndoable(t *testing.T) {
	first, _ := Update(testModel(), DeleteChat{Name: "beta.json"})
	second, _ := Update(first, DeleteChat{Name: "alpha.json"})
	if second.Pending.Name != "alpha.json" || second.Pending.Gen != 2 {
		t.Fatalf("Pending = %+v, want the bar showing alpha.json at gen 2", second.Pending)
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
	model.ChatList = ChatList{"new.json"}
	next, _ := Update(model, NewChat{})
	if next.CurrentChat.Name != "new-2.json" || next.CurrentChat.History != nil {
		t.Fatalf("CurrentChat = %+v, want empty new-2.json", next.CurrentChat)
	}
	if len(next.ChatList) != 2 || next.ChatList[1] != "new-2.json" {
		t.Fatalf("ChatList = %v, want [new.json new-2.json]", next.ChatList)
	}
}

func TestUpdateOpenRenameTargetsChatAndBumpsEpoch(t *testing.T) {
	next, _ := Update(testModel(), OpenRename{Name: "beta.json"})
	if next.Rename.Target != "beta.json" || next.Rename.Epoch != 1 {
		t.Fatalf("Rename = %+v, want beta.json at epoch 1", next.Rename)
	}
	reopened, _ := Update(next, OpenRename{Name: "beta.json"})
	if reopened.Rename.Epoch != 2 {
		t.Fatalf("Epoch = %d, want 2 (bumped on every open)", reopened.Rename.Epoch)
	}
	unknown, _ := Update(testModel(), OpenRename{Name: "nope.json"})
	if unknown.Rename.Target != "" {
		t.Fatalf("Rename = %+v, want closed for unknown chat", unknown.Rename)
	}
}

func TestUpdateRenameChatRenamesCurrentInPlace(t *testing.T) {
	opened, _ := Update(testModel(), OpenRename{Name: "alpha.json"})
	next, _ := Update(opened, RenameChat{To: "ideas"}) // extension added
	if next.ChatList[0] != "ideas.json" || next.ChatList[1] != "beta.json" {
		t.Fatalf("ChatList = %v, want [ideas.json beta.json] (index kept)", next.ChatList)
	}
	if next.CurrentChat.Name != "ideas.json" {
		t.Fatalf("CurrentChat.Name = %q, want ideas.json", next.CurrentChat.Name)
	}
	if len(next.CurrentChat.History) != 1 {
		t.Fatalf("rename must not touch the loaded history")
	}
	if next.Rename.Target != "" {
		t.Fatalf("modal still open: %+v", next.Rename)
	}
}

func TestUpdateRenameChatRejectsInvalidNames(t *testing.T) {
	opened, _ := Update(testModel(), OpenRename{Name: "alpha.json"})

	dup, _ := Update(opened, RenameChat{To: "beta"})
	if dup.ChatList[0] != "alpha.json" || dup.Rename.Target == "" {
		t.Fatalf("duplicate name must be rejected with the modal kept open: %+v", dup.Rename)
	}

	sep, _ := Update(opened, RenameChat{To: "a/b"})
	if sep.ChatList[0] != "alpha.json" || sep.Rename.Target == "" {
		t.Fatalf("path separators must be rejected with the modal kept open: %+v", sep.Rename)
	}

	empty, _ := Update(opened, RenameChat{To: "  "})
	if empty.ChatList[0] != "alpha.json" || empty.Rename.Target != "" {
		t.Fatalf("empty submit must close without renaming: %+v", empty.Rename)
	}

	same, _ := Update(opened, RenameChat{To: "alpha"})
	if same.ChatList[0] != "alpha.json" || same.Rename.Target != "" {
		t.Fatalf("unchanged name must close without renaming: %+v", same.Rename)
	}
}

func TestUpdateChatListReplacesList(t *testing.T) {
	next, _ := Update(testModel(), ChatList{"gamma.json"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "gamma.json" {
		t.Fatalf("ChatList = %v, want [gamma.json]", next.ChatList)
	}
}
