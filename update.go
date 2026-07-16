package main

import (
	"slices"

	"github.com/vibrantgio/mvu"

	openai "github.com/sashabaranov/go-openai"
)

// Update is the MVU update function: it reduces the model and returns the
// command whose messages the runner in main.go feeds back into the loop.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch message := message.(type) {

	case Config:
		model.CurrentChat.Name = message.LastChat
		commands := []mvu.Command{LoadChatList(model.ChatDir()).Trace("Load Chat List")}
		// LastChat is empty when the last chat was deleted; there is no
		// history to load then.
		if message.LastChat != "" {
			commands = append(commands, LoadHist(model.ChatFile(message.LastChat)).Trace("Load History"))
		}
		return model, mvu.DoConcurrent(commands...)

	case Prompt:
		command := mvu.DoNothing()
		if message.Content != "" {
			// Prompting with no chat selected (every chat was deleted)
			// starts a fresh one, so the completion has a file to persist to.
			if model.CurrentChat.Name == "" {
				model.CurrentChat.Name = "new.json"
				if !slices.Contains(model.ChatList, model.CurrentChat.Name) {
					model.ChatList = append(model.ChatList, model.CurrentChat.Name)
				}
			}
			model.CurrentChat.History = append(model.CurrentChat.History, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: message.Content,
			})
			command = mvu.DoSequence(
				RequestChatCompletion(model.AuthToken, model.CurrentChat.History).Trace("Request Chat Completion"),
				SaveConfig(model.ConfigFile(), Config{LastChat: model.CurrentChat.Name}).Trace("Save Config"),
			)
		}
		return model, command

	case openai.ChatCompletionStreamResponse:
		command := mvu.DoNothing()
		if len(message.Choices) > 0 {
			choice := message.Choices[0]

			if choice.Delta.Role != "" {
				model.CurrentChat.History = append(model.CurrentChat.History, openai.ChatCompletionMessage{
					Role:    choice.Delta.Role,
					Content: choice.Delta.Content,
				})
			} else {
				model.CurrentChat.History[len(model.CurrentChat.History)-1].Content += choice.Delta.Content
			}

			if choice.FinishReason == openai.FinishReasonStop {
				command = SaveHist(model.ChatFile(model.CurrentChat.Name), model.CurrentChat.History).Trace("Save History")
			}
		}
		return model, command

	case []openai.ChatCompletionMessage:
		model.CurrentChat.History = message
		return model, mvu.DoNothing()

	case ChatList:
		model.ChatList = message
		return model, mvu.DoNothing()

	case DeleteChat:
		remaining := make(ChatList, 0, len(model.ChatList))
		for _, name := range model.ChatList {
			if name != message.Name {
				remaining = append(remaining, name)
			}
		}
		model.ChatList = remaining
		commands := []mvu.Command{DeleteHist(model.ChatFile(message.Name)).Trace("Delete History")}
		if model.CurrentChat.Name == message.Name {
			// The current chat was deleted: fall back to the first
			// remaining chat, or to the empty state when none are left.
			model.CurrentChat.History = nil
			if len(remaining) > 0 {
				model.CurrentChat.Name = remaining[0]
				commands = append(commands,
					LoadHist(model.ChatFile(remaining[0])).Trace("Load Selected History"),
				)
			} else {
				model.CurrentChat.Name = ""
			}
			commands = append(commands,
				SaveConfig(model.ConfigFile(), Config{LastChat: model.CurrentChat.Name}).Trace("Save Config"),
			)
		}
		return model, mvu.DoConcurrent(commands...)

	case SelectChat:
		if model.CurrentChat.Name == message.Name {
			return model, mvu.DoNothing()
		}
		model.CurrentChat.Name = message.Name
		model.CurrentChat.History = nil
		command := mvu.DoConcurrent(
			LoadHist(model.ChatFile(message.Name)).Trace("Load Selected History"),
			SaveConfig(model.ConfigFile(), Config{LastChat: message.Name}).Trace("Save Config"),
		)
		return model, command
	}
	return model, mvu.DoNothing()
}
