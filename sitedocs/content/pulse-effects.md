# Effects

`pulse/glow` paints vibrancy halos behind accented surfaces and
`pulse/depth` renders soft elevation shadows driven by the prism
`ElevationLevel` token, so visual depth stays consistent with the theme.

`pulse/springbutton` wraps any clickable with spring-loaded press
feedback — the smallest useful composition of the motion primitives.

## Elevation shadow behind a surface

```go
depth.Shadow(gtx, bounds, tokens.Level2)
```

## Accent halo

```go
glow.Halo(gtx, bounds, glow.Options{})
```
