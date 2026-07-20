# Patterns

Cadence is the pattern layer: accordion, alert, breadcrumb, card, feature,
hero, modal, navbar, pagination, popover, pricing, shell, sidebar, table,
tabs, testimonial, toast and tooltip.

Every pattern is a callable function consuming the Prism theme observable
and returning `rx.Observable[layout.Widget]`, with a static `Render`
variant for golden-image tests.

> Source is intentionally short — copy a pattern into your app and modify
> it.

## Subscribe to a pattern

```go
heroObs := hero.Hero(th, hero.Props{
    Title:    "Hello",
    Subtitle: "world",
})
```

## Accordion with a single-open reducer

```go
accObs := accordion.Accordion(th, accordion.Props{
    Sections: secs, Open: openObs,
    OnToggle: toggle,
})
```
