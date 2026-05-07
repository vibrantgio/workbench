# DECISIONS.md — Pre-Phase 0 Architectural Decisions

These decisions are required by DESIGN §"Phase 00 — Validation Experiments" before Phase 0 work begins. Each records the commitment made, the path not taken, and the rationale. They correspond to concerns #4 and #5 in DESIGN §"Architectural Limits & Required Experiments".

---

## Decision 1 — Undo/Redo Strategy (DESIGN §concern #4)

**Chosen path:** FRP apps do not support undo. Apps that require undo/redo (code editors, document editors, drawing tools) use the MVU architecture instead.

**Rejected alternative:** A `MessageOp`-replay buffer that captures all interaction events and replays them to reconstruct prior state in FRP apps.

**Rationale:**

DESIGN §concern #4 notes that pure FRP apps (coinviz style) scatter state across many `Defer` closures and observable accumulators — there is no single snapshottable "current state." A replay buffer would require every input event to be serialisable, all side effects (network, file I/O, time) to be deterministic or excluded from replay, and the entire observable graph to produce identical output for identical event sequences. That engineering cost is substantial.

MVU apps get undo for free: snapshotting the `Model` struct is sufficient. The FRP/MVU architectural split already exists in the codebase; committing to it as an explicit rule makes the constraint clear rather than leaving it implicit.

Apps built with VibrantGIO that require undo must use the MVU pattern. FRP is appropriate for reactive dashboards, visualisations, and instruments where session history is not expected by users.

---

## Decision 2 — Plugin / Extension Systems (DESIGN §concern #5)

**Chosen path:** VibrantGIO has no plugin or extension system. All components are compiled statically into the host application. Extensibility is a non-goal.

**Rejected alternative:** A scripting integration layer (Tengo, Starlark, WASM, or JS via goja) that allows third-party code to extend applications at runtime.

**Rationale:**

DESIGN §concern #5 notes that Go's plugin support is poor cross-platform and the architecture compiles all components statically. The class of apps ruled out by this decision (IDEs with extensions, DAWs with VSTs, apps with user macros/automation) are also constrained by concerns #1 (no component identity) and #4 (no undo in FRP). Adding a scripting layer in isolation would not unlock those apps.

The Phases 0–4 roadmap introduces no scripting primitives. If a VibrantGIO-based application later requires scripting, it can embed Tengo or Starlark at the application layer without any changes to the library itself. The decision to add a scripting module can be made as a new planning goal when a concrete use case justifies the investment.

Committing to "no extensibility" now keeps the architecture focused on its core strengths.
