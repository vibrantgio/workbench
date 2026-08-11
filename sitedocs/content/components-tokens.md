# Tokens & theme

`theme/tokens` holds the semantic scales: `ColorTokens` carries the nine-step
functional `Ramps`, the pinned accent bases with their "On" foregrounds
(`Primary`/`OnPrimary`, `Error`/`OnError`, …) and the thin semantic layer
(`Background`, `Text`, `Surface`, `Divider`), plus `Typography`,
`SpacingScale`, `RadiusScale`, `MotionScale`, `Density` and `ElevationScale`.
`DefaultLight` and `DefaultDark` ship ready to use.

`theme/theme` carries one observable per token category. Components
subscribe to exactly the categories they consume, so a theme change
re-emits only the widgets it affects.

Both packages used to live in `prism`; `prism/tokens` and `prism/theme` were
forwarding shims after the move and were deleted in prism v0.2.0.

## Consume a token category

```go
colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
    return t.Color
})
```

## Always pair grounds with their On colour

```go
paint.ColorOp{Color: c.OnPrimary} // text on Primary
paint.ColorOp{Color: c.OnError}   // text on Error
```

Surfaces have no "On" pin — read their foreground off the neutral ramp, which
is what the deleted `OnSurface` alias always resolved to:

```go
paint.ColorOp{Color: c.Text}                   // body text on Background
paint.ColorOp{Color: c.Ramps.Neutral.Step(900)} // text on Surface
paint.ColorOp{Color: c.Ramps.Neutral.Step(700)} // low-contrast text on Surface
```

## Type roles

`Typography` holds one `TextStyle` per Material Design 3 role —
`DisplayLarge` through `BodySmall`, plus a sixteenth `Code` role on the mono
face. Pass the whole `Typography` to anything that spends several roles, and
a single `TextStyle` to anything that draws one run of text.
