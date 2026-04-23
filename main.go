package main

import (
	"fmt"
	"os"

	"gioui.org/app"

	"github.com/reactivego/mvu"
)

func main() {
	go MindChat()
	app.Main()
}

func MindChat() {
	runtime := mvu.NewRuntime[Model](app.Title("MindChat"), app.Size(1024, 768), app.MinSize(575, 256))
	runtime.Init = Init()
	runtime.Update = Update()
	runtime.View = View()
	if err := runtime.Run(Backdrop()); err == nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	os.Exit(0)
}
