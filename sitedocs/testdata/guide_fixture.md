# Fixture guide

The one document the outline tests read. Its heading skeleton mirrors the
real guide's shape: a lone `#` title that is not a tree root, `##`
sections that are the tree's top level, `###` children that disclose, a
`####` that must never reach the tree, and a childless `##`.

## First section

Prose under the first section. The fenced block and the quote below are
here so a render of this page can be asked whether a raised inset has a
step to stand on: both are drawn against the page's own ground, and on a
page that was itself the fence's colour neither had one.

```go
func step(off page.Ground) tokens.ElevationLevel {
	return off + 1
}
```

> A quote block is marked rather than filled: a Primary bar leads it and
> its prose is set in the low-contrast step.

### First child

Prose under the first child.

### Second child

Prose under the second child.

#### Too deep for the tree

This heading is level four and stays out of the outline.

## Second section, childless

One thought, no children.

## Third section

### Lone child

Prose under the lone child.
