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

func TestUpdateChatListReplacesList(t *testing.T) {
	next, _ := Update(testModel(), ChatList{"gamma.json"})
	if len(next.ChatList) != 1 || next.ChatList[0] != "gamma.json" {
		t.Fatalf("ChatList = %v, want [gamma.json]", next.ChatList)
	}
}
