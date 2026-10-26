package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"unicode"

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
			return model.withStatus(msg.Name, Status{State: Failed, Ended: msg.Ended, Detail: msg.Err}), mvu.DoNothing()
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
// loop: Started once the process spawns, then Exited with Ended set when it
// ends. A start failure sends one Exited with Ended clear and the error.
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
				return Exited{Name: app.Name, Ended: true, Err: exitDetail(err, &stderr)}, nil, false
			}
			return Exited{Name: app.Name, Ended: true}, nil, false
		}
		return nil, nil, true
	})}
}

// exitDetail is the error text of a process that ended with an error: all of
// its stderr as the process wrote it, with only the trailing white space
// removed. Both `go run` compile failures and app panics write their whole
// account there. The exit error itself is the text only when stderr is empty.
func exitDetail(err error, stderr *bytes.Buffer) string {
	if text := strings.TrimRightFunc(stderr.String(), unicode.IsSpace); text != "" {
		return text
	}
	return err.Error()
}
