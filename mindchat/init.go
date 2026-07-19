package main

import (
	"fmt"
	"os"

	"github.com/vibrantgio/mvu"

	_ "github.com/joho/godotenv/autoload"
)

// Init returns the seed Model the message scan starts from and the startup
// command that loads the last-used chat from the config file.
func Init() (Model, mvu.Command) {
	datadir, err := DataDir("nl.simpleapps", "mindchat")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mindchat: data dir:", err)
		os.Exit(1)
	}
	// OPENAI_API_KEY is optional now that providers are configured in the
	// settings modal; when present it seeds the first provider (see the
	// Config reduction).
	authtoken := os.Getenv("OPENAI_API_KEY")
	model := Model{DataDir: datadir, AuthToken: authtoken}
	return model, mvu.DoSequence(
		// Deletes not undone before the previous quit come back first, so
		// the migration sweep and the chat list load see them.
		RestoreTrash(model.TrashDir(), model.ChatDir()).Trace("Restore Trash"),
		// Pre-JSONL chat files convert once, before anything reads them.
		MigrateChats(model.ChatDir()).Trace("Migrate Chats"),
		LoadConfig(model.ConfigFile(), Config{LastChat: "monoid.jsonl"}).Trace("Load Config"),
	)
}
