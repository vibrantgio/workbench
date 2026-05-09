package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"

	"golang.org/x/exp/slices"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"

	openai "github.com/sashabaranov/go-openai"
)

func LoadConfig(filename string, initial Config) mvu.Command {
	loader := rx.Defer(func() rx.Observable[any] {
		if file, err := os.Open(filename); err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			var cfg Config
			if err = decoder.Decode(&cfg); err == nil {
				return rx.Of[any](cfg)
			}
		}
		return rx.Of[any](initial)
	})
	return mvu.Command{Observable: loader}
}

func SaveConfig(filename string, config Config) mvu.Command {
	return mvu.Command{Observable: rx.Create(func(index int) (Next any, Err error, Done bool) {
		file, err := os.Create(filename)
		if err != nil {
			return nil, err, true
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(config)
		if err != nil {
			return nil, err, true
		}
		return nil, nil, true
	})}
}

func RequestChatCompletion(ctx context.Context, authToken string, hist []openai.ChatCompletionMessage) mvu.Command {
	messages := slices.Clone(hist)
	return mvu.Command{Observable: rx.Defer(func() rx.Observable[any] {
		client := openai.NewClient(authToken)
		request := openai.ChatCompletionRequest{
			Model:    "gpt-5.5",
			Messages: messages,
			// MaxTokens:        20,
			// Temperature:      1.0,
			// TopP:             1.0,
			// N:                1,
			Stream: true,
			// Stop:             []string{},
			// PresencePenalty:  1.0,
			// FrequencyPenalty: 1.0,
			// LogitBias:        map[string]int{},
			// User:             "",
			// Functions:        []*openai.FunctionDefine{},
			// FunctionCall:     "",
		}
		stream, err := client.CreateChatCompletionStream(ctx, request)
		if err != nil {
			return rx.Throw[any](err)
		}
		return rx.Create(func(index int) (Next any, Err error, Done bool) {
			Next, Err = stream.Recv()
			if Err != nil {
				if errors.Is(Err, io.EOF) {
					Err = nil
				}
				Done = true
			}
			return
		})
	})}
}

func LoadHist(filename string) mvu.Command {
	return mvu.Command{Observable: rx.Create(func(index int) (Next any, Err error, Done bool) {
		if index == 0 {
			file, err := os.Open(filename)
			if err != nil {
				return nil, err, true
			}
			defer file.Close()
			decoder := json.NewDecoder(file)
			var hist []openai.ChatCompletionMessage
			err = decoder.Decode(&hist)
			if err != nil {
				return nil, err, true
			}
			return hist, nil, false
		}
		return nil, nil, true
	})}
}

func SaveHist(filename string, hist []openai.ChatCompletionMessage) mvu.Command {
	return mvu.Command{Observable: rx.Create(func(index int) (Next any, Err error, Done bool) {
		file, err := os.Create(filename)
		if err != nil {
			return nil, err, true
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(hist)
		if err != nil {
			return nil, err, true
		}
		return nil, nil, true
	})}
}

func LoadChatList(directory string) mvu.Command {
	chats := rx.Scan(Directory(directory), []fs.DirEntry(nil), func(acc []fs.DirEntry, entry fs.DirEntry) []fs.DirEntry {
		return append(acc, entry)
	})
	return mvu.Command{Observable: rx.Map(chats, func(entries []fs.DirEntry) any {
		var names ChatList
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return names
	})}
}
