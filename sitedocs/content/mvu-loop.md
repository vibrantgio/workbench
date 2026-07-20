# The loop

`mvu` is the Model-View-Update runtime: your application is a model,
messages describe what happened, and a pure `Update` reduces each message
into the next model plus an optional `Command` for async work.

`mvu.Loop` folds the window's message stream over `Update` and emits every
model; `MessageOp` lets any widget emit a message from layout code,
delivered in the same frame as the click that produced it.

## Run the loop

```go
init := func() (Model, mvu.Command) {
    return initialModel(), mvu.DoNothing()
}
models, runner := mvu.Loop(win.Messages(), init, Update)
```

## Emit a message from layout code

```go
mvu.MessageOp{Message: SetRoute{Page: pageAbout}}.Add(gtx.Ops)
```
