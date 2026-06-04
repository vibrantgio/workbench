# Performance Baseline — coinviz (pre-Prism)

Captured as part of milestone **G−1.6** before any Prism work begins.
All numbers anchor future regression comparisons.

## Platform

| Field | Value |
|---|---|
| CPU | Apple M1 |
| OS | macOS 15.7.4 (Sequoia) |
| Go | go1.25.5 darwin/arm64 |
| Gio | v0.9.0 |
| Date | 2026-05-07 |

## Method

Benchmarks live in `coinviz/bench/baseline_test.go`.

Load scenario: **BTC-USDT, 1h candles, 500 samples, 1200×800 px, 1 dp/px, pointer off-screen**.

Each benchmark calls `pane.Panes.Render(gtx, bounds, margin, style, scroll, pointer, data)`
in a tight `b.N` loop, resetting `op.Ops` between frames.  Timing is op-buffer recording
only; no GPU submission.

Run command:

```
go test -bench=. -benchmem -benchtime=3s ./bench/
```

## Per-pane results

| Pane | ns/op | B/op | allocs/op | Notes |
|---|---:|---:|---:|---|
| chart | 149 225 | 25 708 | 35 | candlestick + ema(8) + sma(43) + sma(128) + bollinger(10) |
| volume | 79 844 | 9 059 | 7 | |
| rsi | 24 862 | 8 653 | 14 | rsi(14) + rsi(24) |
| obv | 14 035 | 4 500 | 5 | |
| splitvolume | 77 053 | 1 774 | 7 | |
| macd | 92 735 | 14 084 | 14 | |
| htsine | 209 308 | 10 413 | 10 | |
| htphasor | 43 749 | 9 093 | 8 | |
| httrendmode | 206 508 | 6 264 | 8 | |
| htphase | 201 468 | 8 056 | 9 | |
| htperiod | 36 181 | 7 636 | 8 | |
| timebar | 470 | 0 | 0 | |

## All-panes combined (12 panes)

| ns/op | B/op | allocs/op |
|---:|---:|---:|
| 1 205 146 | 98 204 | 115 |

Rendering all 12 non-backtest pane types in a single frame takes **~1.2 ms** of
op-recording time.  The DESIGN.md layout-work budget is 8.3 ms; this leaves
~7 ms of headroom before GPU submission and present.

## Observations

- **Hilbert-transform panes** (htsine, httrendmode, htphase) dominate at ~200 µs each
  due to the O(N) DSP computation executed inside every Render call.  These are
  candidates for memoisation in the `Defer` scope (DESIGN §Performance).
- **chart** costs ~149 µs with a full indicator stack; indicator `Compute` allocates
  per-call (EMA, SMA, Bollinger each `make([]float64, N)`).
- **timebar** is the sole zero-allocation pane.
- **backtest** pane is not included: its `Render` calls `log.Println` on every invocation
  when `backtest.StrategyFactory` is uninitialized, which would inflate timing with I/O.
  A follow-up can add a minimal strategy fixture to enable that measurement.
- **PARALLELISM**: benchmarks run with `-cpu 8` (GOMAXPROCS default on M1); individual
  pane benchmarks are serial per b.N loop — no goroutine contention.

## Raw output

```
goos: darwin
goarch: arm64
pkg: github.com/xpt-nl/coinviz/bench
cpu: Apple M1
BenchmarkPane_chart-8           24652    149225 ns/op    25708 B/op    35 allocs/op
BenchmarkPane_volume-8          43986     79844 ns/op     9059 B/op     7 allocs/op
BenchmarkPane_rsi-8            146025     24862 ns/op     8653 B/op    14 allocs/op
BenchmarkPane_obv-8            259448     14035 ns/op     4500 B/op     5 allocs/op
BenchmarkPane_splitvolume-8     46330     77053 ns/op     1774 B/op     7 allocs/op
BenchmarkPane_macd-8            38914     92735 ns/op    14084 B/op    14 allocs/op
BenchmarkPane_htsine-8          17220    209308 ns/op    10413 B/op    10 allocs/op
BenchmarkPane_htphasor-8        82020     43749 ns/op     9093 B/op     8 allocs/op
BenchmarkPane_httrendmode-8     17449    206508 ns/op     6264 B/op     8 allocs/op
BenchmarkPane_htphase-8         18531    201468 ns/op     8056 B/op     9 allocs/op
BenchmarkPane_htperiod-8       102517     36181 ns/op     7636 B/op     8 allocs/op
BenchmarkPane_timebar-8       7452284       469 ns/op        0 B/op     0 allocs/op
BenchmarkAllPanes-8              3123   1205146 ns/op    98204 B/op   115 allocs/op
PASS
ok      github.com/xpt-nl/coinviz/bench    59.432s
```

---

# Phase 1 component baseline — Prism

Captured as part of milestone **GX.2** (DESIGN §"Performance — Methodology —
Benchmark harness"). Each Phase 1 component plugs its `*_bench_test.go` into the
shared `prism/bench` harness (`BenchFrame(b, widget)`), which drives
`widget(gtx)` under synthesized constraints, resets the op buffer per frame, and
enables `b.ReportAllocs()`. These numbers anchor the per-component >5% regression
rule on **both** ns/op and B/op.

## Platform

| Field | Value |
|---|---|
| CPU | Apple M1 |
| OS | macOS 15.7.7 (24G720) |
| Go | go1.25.5 darwin/arm64 |
| Gio | v0.9.0 |
| Date | 2026-06-04 |

Run command (from workspace root):

```
go test -bench=. -benchmem -benchtime=3s ./prism/button/... ./prism/input/... ./prism/list/...
```

(The DESIGN/PLAN measurable lists `./prism/input/textfield/...`, but textfield
ships in the `prism/input` module — there is no `textfield` subpackage — so the
input module is addressed as `./prism/input/...`.)

## Per-component results

| Component | Benchmark | Scenario | ns/op | B/op | allocs/op |
|---|---|---|---:|---:|---:|
| button | `BenchmarkButtonRender` | idle render, static unfocused | 1 165 | 0 | 0 |
| input/textfield | `BenchmarkTextFieldCaretBlink` | focused live editor, caret laid out (typing hot path) | 5 547 | 328 | 7 |
| list | `BenchmarkListLayout/N=1000` | 1000-item virtual list, ~5 rows visible | 675 | 0 | 0 |

## Notes

- **button** and **list** are zero-allocation per frame. The list row is taken
  from the `N=1000` sub-case; ns/op stays flat (~675 ns) from `N=10` to
  `N=10000`, confirming O(visible) layout cost rather than O(total).
- **textfield/caret-blink is intentionally not apples-to-apples** with the
  static rows. It measures the live `widget.Editor` path — editor layout, caret
  geometry, and the `input.Router` frame — with the editor focused and holding
  text: the focused-editor frame a blinking cursor is rendered into. `gtx.Now`
  is the zero time on every iteration, so this is one frozen focused frame
  repeated — a stable, reproducible anchor; the exact caret blink phase within
  it is not asserted, only that the focused path runs. The static
  `BenchmarkTextFieldRender` (unfocused placeholder) measures 0 B/op for
  contrast. The bench fails loudly via an `OnChange` guard if focus ever fails
  to take, so the row can never silently regress to the cheaper unfocused frame.
