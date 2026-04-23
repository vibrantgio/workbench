package main

import (
	"context"

	"github.com/reactivego/mvu"

	openai "github.com/sashabaranov/go-openai"
)

func Update() func(Model, mvu.Message) (Model, mvu.Command) {
	ctx := context.Background()
	return func(model Model, message mvu.Message) (Model, mvu.Command) {
		switch message := message.(type) {

		case Config:
			model.CurrentChat.Name = message.LastChat
			command := mvu.DoConcurrent(
				LoadHist(model.ChatFile(message.LastChat)).Trace("Load History"),
				LoadChatList(model.ChatDir()).Trace("Load Chat List"),
			)
			return model, command

		case Prompt:
			command := mvu.DoNothing()
			if message.Content != "" {
				model.CurrentChat.History = append(model.CurrentChat.History, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: message.Content,
				})
				command = mvu.DoSequence(
					RequestChatCompletion(ctx, model.AuthToken, model.CurrentChat.History).Trace("Request Chat Completion"),
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
}
