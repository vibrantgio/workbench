# Live theme

`spectrum/transition` animates token changes so a dark-mode flip
cross-fades instead of snapping: `ColorTokensTween` interpolates a full
`ColorTokens` set over a fixed frame budget.

`spectrum/preferences` persists the user's explicit appearance choice
across launches, stored under the OS config directory for the app.

## Tween between token sets

```go
tw := transition.ColorTokensTween(fromTokens, toTokens, 12)
```

## Where preferences live

```go
path, err := preferences.Path("myapp")
```
