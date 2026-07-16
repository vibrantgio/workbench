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
				model.CurrentChat.Name = FreshChatName(append(slices.Clone(model.ChatList), model.Pending.Name))
				model.ChatList = append(slices.Clone(model.ChatList), model.CurrentChat.Name)
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
		var commands []mvu.Command
		// Only one undo window at a time: a second delete finalises the
		// first immediately.
		if model.Pending.Name != "" {
			commands = append(commands, DeleteHist(model.ChatFile(model.Pending.Name)).Trace("Delete History"))
			model.Pending = PendingDelete{}
		}
		index := slices.Index(model.ChatList, message.Name)
		if index < 0 {
			return model, mvu.DoConcurrent(commands...)
		}
		model.ChatList = slices.Delete(slices.Clone(model.ChatList), index, index+1)
		wasCurrent := model.CurrentChat.Name == message.Name
		// The delete is SOFT: the history file stays on disk until the
		// undo window closes (ConfirmDelete), so UndoDelete can restore it.
		model.DeleteGen++
		model.Pending = PendingDelete{Name: message.Name, Index: index, WasCurrent: wasCurrent, Gen: model.DeleteGen}
		commands = append(commands, ExpireDelete(model.DeleteGen, UndoWindow).Trace("Undo Window"))
		if wasCurrent {
			// The current chat was deleted: fall back to the first
			// remaining chat, or to the empty state when none are left.
			model.CurrentChat.History = nil
			if len(model.ChatList) > 0 {
				model.CurrentChat.Name = model.ChatList[0]
				commands = append(commands,
					LoadHist(model.ChatFile(model.CurrentChat.Name)).Trace("Load Selected History"),
				)
			} else {
				model.CurrentChat.Name = ""
			}
			commands = append(commands,
				SaveConfig(model.ConfigFile(), Config{LastChat: model.CurrentChat.Name}).Trace("Save Config"),
			)
		}
		return model, mvu.DoConcurrent(commands...)

	case UndoDelete:
		if model.Pending.Name == "" {
			return model, mvu.DoNothing()
		}
		pending := model.Pending
		model.Pending = PendingDelete{}
		index := min(pending.Index, len(model.ChatList))
		model.ChatList = slices.Insert(slices.Clone(model.ChatList), index, pending.Name)
		if pending.WasCurrent {
			model.CurrentChat.Name = pending.Name
			model.CurrentChat.History = nil
			return model, mvu.DoConcurrent(
				LoadHist(model.ChatFile(pending.Name)).Trace("Load Restored History"),
				SaveConfig(model.ConfigFile(), Config{LastChat: pending.Name}).Trace("Save Config"),
			)
		}
		return model, mvu.DoNothing()

	case ConfirmDelete:
		// Ignore stale timers: the pending delete was undone or superseded.
		if model.Pending.Name == "" || model.Pending.Gen != message.Gen {
			return model, mvu.DoNothing()
		}
		name := model.Pending.Name
		model.Pending = PendingDelete{}
		return model, DeleteHist(model.ChatFile(name)).Trace("Delete History")

	case NewChat:
		name := FreshChatName(append(slices.Clone(model.ChatList), model.Pending.Name))
		model.ChatList = append(slices.Clone(model.ChatList), name)
		model.CurrentChat = Chat{Name: name}
		return model, mvu.DoConcurrent(
			SaveHist(model.ChatFile(name), []openai.ChatCompletionMessage{}).Trace("Create Chat"),
			SaveConfig(model.ConfigFile(), Config{LastChat: name}).Trace("Save Config"),
		)

	case OpenSettings:
		// Settings surface (OPENAI_API_KEY configuration) not built yet.
		return model, mvu.DoNothing()

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
