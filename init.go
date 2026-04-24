package main

import (
	"fmt"
	"os"

	"github.com/vibrantgio/mvu"

	_ "github.com/joho/godotenv/autoload"
)

func Init() func() (Model, mvu.Command) {
	datadir, err := DataDir("nl.simpleapps", "mindchat")
	if err != nil {
		fmt.Println("DataDir Error:", err)
		os.Exit(1)
	}
	authtoken, ok := os.LookupEnv("OPENAI_AUTH_TOKEN")
	if !ok {
		fmt.Println("No OPENAI_AUTH_TOKEN in environment")
		os.Exit(1)
	}
	return func() (Model, mvu.Command) {
		model := Model{DataDir: datadir, AuthToken: authtoken}
		return model, LoadConfig(model.ConfigFile(), Config{LastChat: "monoid.json"}).Trace("Load Config")
	}
}
