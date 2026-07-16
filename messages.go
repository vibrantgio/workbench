package main

import openai "github.com/sashabaranov/go-openai"

type LoadState struct{}

type Config struct {
	LastChat string
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

type Chat struct {
	Name    string
	History []openai.ChatCompletionMessage
}
