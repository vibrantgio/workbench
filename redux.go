package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// Update folds launch lifecycle messages into the Model. Launch is the only
// message with a side effect: it returns the streaming launch command below.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch msg := message.(type) {
	case Launch:
		if s := model.StatusOf(msg.Name); s.State == Starting || s.State == Running {
			return model, mvu.DoNothing()
		}
		app, ok := findApp(msg.Name)
		if !ok {
			return model, mvu.DoNothing()
		}
		return model.withStatus(msg.Name, Status{State: Starting}), launchCommand(app)
	case Started:
		return model.withStatus(msg.Name, Status{State: Running}), mvu.DoNothing()
	case Exited:
		if msg.Err != "" {
			return model.withStatus(msg.Name, Status{State: Failed, Detail: msg.Err}), mvu.DoNothing()
		}
		return model.withStatus(msg.Name, Status{}), mvu.DoNothing()
	}
	return model, mvu.DoNothing()
}

// Init returns the seed Model; nothing is running at startup.
func Init() (Model, mvu.Command) {
	return Model{}, mvu.DoNothing()
}

func findApp(name string) (App, bool) {
	for _, a := range Apps {
		if a.Name == name {
			return a, true
		}
	}
	return App{}, false
}

// launchCommand starts app with `go` and streams two messages through the MVU
// loop: Started once the process spawns, then Exited when it ends. A start
// failure collapses to a single Exited carrying the error.
func launchCommand(app App) mvu.Command {
	var cmd *exec.Cmd
	var stderr bytes.Buffer
	failed := false
	return mvu.Command{Observable: rx.Create[mvu.Message](func(index int) (mvu.Message, error, bool) {
		switch index {
		case 0:
			inv, err := launchInvocation(app)
			if err == nil {
				cmd = exec.Command("go", inv.Args...)
				cmd.Dir = inv.Dir
				cmd.Stdout = os.Stdout
				cmd.Stderr = &stderr
				err = cmd.Start()
			}
			if err != nil {
				failed = true
				return Exited{Name: app.Name, Err: err.Error()}, nil, false
			}
			return Started{Name: app.Name}, nil, false
		case 1:
			if failed {
				return nil, nil, true
			}
			if err := cmd.Wait(); err != nil {
				return Exited{Name: app.Name, Err: exitDetail(err, &stderr)}, nil, false
			}
			return Exited{Name: app.Name}, nil, false
		}
		return nil, nil, true
	})}
}

// exitDetail folds the last stderr line into the exit error: that line is
// where both `go run` compile failures and app panics say what went wrong.
func exitDetail(err error, stderr *bytes.Buffer) string {
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return err.Error()
	}
	return fmt.Sprintf("%s: %s", err, last)
}
