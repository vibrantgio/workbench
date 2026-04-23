package main

import (
	"path/filepath"
)

type Model struct {
	DataDir     string
	AuthToken   string
	CurrentChat Chat
	ChatList    ChatList
}

func (model Model) ConfigFile() string {
	return filepath.Join(model.DataDir, "config.json")
}

func (model Model) ChatDir() string {
	return filepath.Join(model.DataDir, "chats")
}

func (model Model) ChatFile(name string) string {
	return filepath.Join(model.DataDir, "chats", name)
}
