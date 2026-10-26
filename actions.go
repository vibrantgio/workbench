package main

// Launch starts the named app. Ignored while it is already starting/running.
type Launch struct {
	Name string
}

// Started reports that the app's `go run` process spawned successfully.
type Started struct {
	Name string
}

// Exited reports that the app's process ended, or that it could not be
// opened. Err is "" for a clean exit. Ended is true when the process ran, and
// false when the process could not be opened.
type Exited struct {
	Name  string
	Ended bool
	Err   string
}
