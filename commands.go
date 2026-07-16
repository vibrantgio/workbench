package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

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

func RequestChatCompletion(authToken string, hist []openai.ChatCompletionMessage) mvu.Command {
	messages := slices.Clone(hist)
	return mvu.Command{Observable: rx.Defer(func() rx.Observable[any] {
		ctx := context.Background()
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

// TrashHist moves a chat's history file into the trash directory, where it
// stays undoable. It emits no message; the model was already reduced.
func TrashHist(filename, trashname string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		if err := os.MkdirAll(filepath.Dir(trashname), 0o755); err != nil {
			return nil, err
		}
		return nil, os.Rename(filename, trashname)
	})
}

// RestoreTrash moves every file left in the trash back into the chats
// directory — deletes not undone before the previous quit reappear rather
// than silently vanishing. Runs before LoadConfig at startup. A name that
// meanwhile exists again keeps the live file; the trash copy is dropped.
func RestoreTrash(trashdir, chatdir string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		entries, err := os.ReadDir(trashdir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			target := filepath.Join(chatdir, entry.Name())
			if _, err := os.Stat(target); err == nil {
				_ = os.Remove(filepath.Join(trashdir, entry.Name()))
				continue
			}
			if err := os.Rename(filepath.Join(trashdir, entry.Name()), target); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
}

// RenameHist moves a chat's history file to its new name. It emits no
// message; the model was already reduced when the command was issued.
func RenameHist(oldname, newname string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return nil, os.Rename(oldname, newname)
	})
}

// UndoWindow is how long the undo bar stays visible. It is display-only:
// Cmd/Ctrl-Z keeps working for the whole session (the file sits in the
// trash), the bar just stops advertising it.
const UndoWindow = 15 * time.Second

// ExpireDelete hides a delete's undo bar after the delay. The generation
// guards against the timer of a delete whose bar was replaced or dismissed
// in the meantime. rx.Timer (not time.Sleep) keeps the command cancellable,
// so quitting the app mid-window does not block the runner's teardown.
func ExpireDelete(gen int, after time.Duration) mvu.Command {
	return mvu.Command{Observable: rx.Map(rx.Timer[int](after), func(int) any {
		return ConfirmDelete{Gen: gen}
	})}
}

func LoadChatList(directory string) mvu.Command {
	chats := rx.Scan(Directory(directory), []fs.DirEntry(nil), func(acc []fs.DirEntry, entry fs.DirEntry) []fs.DirEntry {
		return append(acc, entry)
	})
	return mvu.Command{Observable: rx.Map(chats, func(entries []fs.DirEntry) any {
		var names ChatList
		for _, entry := range entries {
			// Skip directories — notably .trash, which holds undoable
			// deletes, not live chats.
			if entry.IsDir() {
				continue
			}
			names = append(names, entry.Name())
		}
		return names
	})}
}
