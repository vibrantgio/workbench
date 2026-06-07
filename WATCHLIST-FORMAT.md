# WATCHLIST-FORMAT.md — the vibrantgio watchlists file format

This document specifies the on-disk JSON format the `vibrantgio/watchlist`
editor reads and writes, and the platform path convention for locating it. It
is the contract a `coinviz` adoption (or any other consumer) can implement
against without reading the editor's source.

## File location

The watchlists file lives in the per-user application-config directory, under a
`vibrantgio` subdirectory, named `watchlists.json`. The base directory is
resolved with Go's `os.UserConfigDir()`, which gives the platform-native
location:

| Platform | Path |
| --- | --- |
| macOS   | `~/Library/Application Support/vibrantgio/watchlists.json` |
| Linux   | `$XDG_CONFIG_HOME/vibrantgio/watchlists.json`, defaulting to `~/.config/vibrantgio/watchlists.json` when `XDG_CONFIG_HOME` is unset |
| Windows | `%AppData%\vibrantgio\watchlists.json` (i.e. `C:\Users\<user>\AppData\Roaming\vibrantgio\watchlists.json`) |

Consumers SHOULD use their platform's equivalent of `os.UserConfigDir()` rather
than hard-coding any single path, so the same file is found across tools.

## Top-level shape

The document is a single flat JSON object — not a database, not a binary blob —
with three top-level keys:

```json
{
  "version": 1,
  "watchlists": [ ... ],
  "selected": "default"
}
```

- **`version`** (integer, required) — the format version. The current version
  is `1`. A consumer that reads a `version` it does not recognise SHOULD refuse
  to overwrite the file (to avoid clobbering data written by a newer tool) and
  MAY fall back to read-only/empty behaviour. Writers MUST emit the version
  they wrote against.
- **`watchlists`** (array, required) — an **ordered** list of watchlist
  objects (see below). Order is significant and preserved: the sidebar renders
  the watchlists top-to-bottom in array order, and round-tripping the file
  preserves that order. The top level is deliberately an array rather than a
  JSON object keyed by name, because object key order is not guaranteed by the
  JSON spec and Go map iteration is randomized — an array keeps display order
  stable and deterministic.
- **`selected`** (string, optional) — the `name` of the watchlist the editor
  should pre-select on open. If absent, empty, or naming a watchlist that is
  not present, the consumer SHOULD select the first watchlist in the array (or
  none, if the array is empty).

An absent file and a present file with an empty `watchlists` array are distinct
states: an absent file triggers first-run starter creation (see below); a
present-but-empty file is a valid document that renders the empty-state message.

## Watchlist object

Each entry in `watchlists` is an object:

```json
{
  "name": "default",
  "symbols": [ ... ]
}
```

- **`name`** (string, required) — the watchlist's display name, shown in the
  sidebar. Names SHOULD be unique within a document; if duplicates occur, a
  consumer MAY disambiguate by array index but the `selected` field can only
  address the first match.
- **`symbols`** (array, required) — an **ordered** list of symbol objects.
  Order is significant and preserved.

## Symbol object

Each entry in a watchlist's `symbols` is an object with four fields:

```json
{
  "symbol": "BTC/USD",
  "exchange": "Coinbase",
  "timeframe": "1h",
  "notes": "primary position"
}
```

| Field | JSON key | Type | Required | Meaning |
| --- | --- | --- | --- | --- |
| Symbol    | `symbol`    | string | **required** | The instrument identifier, e.g. `BTC/USD`. The format is consumer-defined (`BASE/QUOTE` is the convention used by the starter data) and not validated by this format. |
| Exchange  | `exchange`  | string | optional | The venue the symbol is tracked on, e.g. `Coinbase`. Empty string or omitted means unspecified. |
| Timeframe | `timeframe` | string | optional | The chart/candle interval, e.g. `1h`, `4h`, `1d`. Empty string or omitted means unspecified. |
| Notes     | `notes`     | string | optional | Free-form user notes about the symbol. Empty string or omitted means none. |

Optional fields default to the empty string when absent. Writers MAY omit
empty optional fields or emit them as `""`; readers MUST treat both the same.

## Complete example

```json
{
  "version": 1,
  "selected": "default",
  "watchlists": [
    {
      "name": "default",
      "symbols": [
        { "symbol": "BTC/USD", "exchange": "Coinbase", "timeframe": "1h", "notes": "" },
        { "symbol": "ETH/USD", "exchange": "Coinbase", "timeframe": "1h", "notes": "" },
        { "symbol": "SOL/USD", "exchange": "Coinbase", "timeframe": "1h", "notes": "" }
      ]
    }
  ]
}
```

## First-run starter

When the file is absent at startup, the editor writes the example document
above (a single watchlist named `default` containing `BTC/USD`, `ETH/USD`, and
`SOL/USD`) so the app always has data to display on first launch. The starter
is written exactly once: a present file — including a present-but-empty one — is
never overwritten by the starter logic.
