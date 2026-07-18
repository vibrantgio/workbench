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
	authtoken, ok := os.LookupEnv("OPENAI_API_KEY")
	if !ok {
		fmt.Fprintln(os.Stderr, "mindchat: no OPENAI_API_KEY in environment")
		os.Exit(1)
	}
	model := Model{DataDir: datadir, AuthToken: authtoken}
	return model, mvu.DoSequence(
		// Deletes not undone before the previous quit come back first, so
		// the chat list load sees them.
		RestoreTrash(model.TrashDir(), model.ChatDir()).Trace("Restore Trash"),
		LoadConfig(model.ConfigFile(), Config{LastChat: "monoid.json"}).Trace("Load Config"),
	)
}
