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

func TestUpdateChatListReplacesList(t *testing.T) {
	next, _ := Update(testModel(), ChatList{"gamma.json"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "gamma.json" {
		t.Fatalf("ChatList = %v, want [gamma.json]", next.ChatList)
	}
}
