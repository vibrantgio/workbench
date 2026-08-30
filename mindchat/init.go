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
	// OPENAI_API_KEY is optional — providers are configured in the settings
	// modal — and when present it seeds the first provider.
	return InitIn(datadir, os.Getenv("OPENAI_API_KEY"))
}

// InitIn is Init's whole body once the data directory is resolved: the seed
// Model and the startup command sequence, for an arbitrary directory. Init
// takes the OS one; a test takes an empty one, which is what a fresh install
// actually is — no chats/, no config.json, nothing.
func InitIn(datadir, authtoken string) (Model, mvu.Command) {
	model := Model{DataDir: datadir, AuthToken: authtoken}
	return model, mvu.DoSequence(
		// Nothing else creates chats/, and everything below reads or writes
		// through it, so it is made before the first of them runs.
		EnsureChatDir(model.ChatDir()).Trace("Ensure Chat Dir"),
		// Deletes not undone before the previous quit come back first, so
		// the migration sweep and the chat list load see them.
		RestoreTrash(model.TrashDir(), model.ChatDir()).Trace("Restore Trash"),
		// Pre-JSONL chat files convert once, before anything reads them.
		MigrateChats(model.ChatDir()).Trace("Migrate Chats"),
		// The fallback config is what a fresh install gets, so it names no
		// last chat: there are none.
		LoadConfig(model.ConfigFile(), Config{}).Trace("Load Config"),
	)
}
