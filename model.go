package main

// LaunchState is the lifecycle of one launched app.
type LaunchState int

const (
	Idle     LaunchState = iota // not running; the default zero value
	Starting                    // `go run` spawned, still compiling/opening
	Running                     // process alive
	Failed                      // last run exited with an error
)

// Status is the per-app slice of the Model.
type Status struct {
	State LaunchState
	// Ended is true when the process ran and ended with an error, and false
	// when the process could not be opened. It applies only when State is
	// Failed.
	Ended  bool
	Detail string // the error text as the process gave it, when State is Failed
}

// Model is the single application state: the status of every launchable app,
// keyed by App.Name. A nil map reads as all-Idle; reducers copy-on-write.
type Model struct {
	Status map[string]Status
}

// StatusOf returns the status for the named app (zero value = Idle).
func (m Model) StatusOf(name string) Status {
	return m.Status[name]
}

// withStatus returns a copy of the model with one app's status replaced.
func (m Model) withStatus(name string, s Status) Model {
	next := make(map[string]Status, len(m.Status)+1)
	for k, v := range m.Status {
		next[k] = v
	}
	next[name] = s
	return Model{Status: next}
}
