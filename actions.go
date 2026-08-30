package main

// Launch starts the named app. Ignored while it is already starting/running.
type Launch struct {
	Name string
}

// Started reports that the app's `go run` process spawned successfully.
type Started struct {
	Name string
}

// Exited reports that the app's process ended. Err is "" for a clean exit.
type Exited struct {
	Name string
	Err  string
}
