# Motion

`pulse/tween` interpolates values over a fixed frame budget and
`pulse/spring` integrates critically-damped physics for interruptible
motion; `pulse/motion` composes them into enter/exit choreography for
appearing and disappearing widgets.

`pulse/conductor` is the shared clock: concurrently running animations
stay phase-coherent, and frame invalidation stops the moment everything
has settled.

## A spring toward a target

```go
s := spring.New(0, 1, spring.Options{})
```
