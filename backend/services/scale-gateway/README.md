# scale-gateway

A local service that talks to a physical price-computing scale (e.g. a
Bizerba scale, connected via a serial-to-Ethernet adapter) and exposes a
small, protocol-agnostic HTTP API for the rest of the system to use. Callers
send a price per kg and get back the approved transaction (weight, total
price) that the scale itself computed and displayed to the customer — this
service never computes a price itself.

It's meant to run on-site, on the same local network as the scale(s) it
talks to, since the scale's TCP endpoint (behind the serial-to-Ethernet
adapter) generally isn't reachable from a cloud backend without extra
networking (VPN/port-forwarding) that isn't practical for a market stall.

## Why a driver abstraction

Bizerba (and other scale vendors) expose more than one way to integrate:
a low-level message protocol (Dialog 02/04, spoken directly over a raw TCP
socket) and a higher-level SDK (RIK, the Retail Integrators Kit). This
service is built so both can be supported side by side, selected per scale
via config, without the rest of the system caring which one a given scale
uses.

- `internal/protocol` — wire-level framing and message codecs (currently
  Dialog 02 and Dialog 04).
- `internal/driver` — the `ScaleDriver` interface (`Connect` /
  `SendPriceAndAwaitTransaction` / `Close`) plus concrete implementations.
  `DialogTCPDriver` implements it today over a raw TCP socket. A RIK-based
  driver is planned to implement the same interface later — `driver.New`
  already recognizes `KindRIK` in config and returns a clear "not
  implemented yet" error until that lands, so switching a scale over once
  it exists is a one-line config change, not a code change.
- `internal/gateway` — the HTTP server and JSON config loading that wire
  configured scales to their drivers.

## The Dialog 02/04 protocol, as implemented here

**This section documents an inferred protocol layout, not an official
specification.** It was reverse-engineered from four example frames and one
description of the checksum algorithm's rationale. There is no confirmed
official Bizerba protocol document backing these field widths — they should
be validated against real hardware (or real vendor documentation) before
this driver is trusted in production. Encoding/decoding is isolated behind
`protocol.Codec` specifically so that if the real field layout differs,
fixing it means changing `dialog02.go`/`dialog04.go` alone.

### Framing

Every message is: `STX <payload> ETX <BCC>` where `STX = 0x02` and
`ETX = 0x03`.

**BCC** (block check character) is the XOR of every payload byte followed
by XOR with `ETX`. **STX is excluded** from the checksum (its value is
constant and carries no information); **ETX is included** so that a
transmission fault which truncates the frame before reaching ETX is caught
immediately rather than silently accepted.

### Dialog 02 (narrow fields)

| Message | Layout | Example |
|---|---|---|
| Set price (request) | 5-digit price, cents, zero-padded | `01499` → $14.99 |
| Transaction (response) | 1-digit status + 4-digit weight (grams) + 4-digit price (cents) | `112501874` → status `1`, 1.250 kg, $18.74 |

### Dialog 04 (wide fields)

Same semantics as Dialog 02, with wider fields (more headroom for larger
prices, and a longer status field):

| Message | Layout | Example |
|---|---|---|
| Set price (request) | 7-digit price, cents, zero-padded | `0001499` → $14.99 |
| Transaction (response) | 3-digit status + 4-digit weight (grams) + 6-digit price (cents) | `1001250001874` → status `100`, 1.250 kg, $18.74 |

**The status field's meaning is unknown** (success/error/pending codes,
a scale ID, or something else) — it's surfaced as an opaque string on
`protocol.TransactionResult.StatusCode` rather than interpreted.

## Config

The service reads a JSON config file (default `config.json`, override with
`-config`). See `config.example.json`:

```json
{
  "listen_address": ":8080",
  "scales": [
    {
      "id": "stall-1-scale-1",
      "kind": "dialog_raw_tcp",
      "dialog_variant": "02",
      "address": "192.168.1.50:9999"
    }
  ]
}
```

- `kind`: `dialog_raw_tcp` (implemented) or `rik` (accepted by config
  validation, but building a driver for it fails until the RIK driver is
  implemented).
- `dialog_variant`: `"02"` or `"04"`, required when `kind` is
  `dialog_raw_tcp`.
- `address`: `host:port` of the scale's TCP endpoint (typically the
  serial-to-Ethernet adapter), required when `kind` is `dialog_raw_tcp`.

## HTTP API

- `GET /scales` — list configured scales with their connection status.
  `held_by_id`/`held_by_name` are present while a vendor holds an active
  claim (see below).
- `POST /scales/{id}/transactions` — body
  `{"price_per_kg_cents": 1499, "holder_id": "vendor-1"}`, returns
  `{"scale_id", "status_code", "weight_grams", "price_cents"}` on success.
  Transactions against a single scale are serialized: only one can be in
  flight at a time. `holder_id` must match the scale's current claim holder
  (or the scale must be unclaimed) — a mismatch returns 409.
- `POST /scales/{id}/claim` — body
  `{"holder_id": "vendor-1", "holder_name": "Alice"}` (`holder_name`
  optional, used only for display). Grants exclusive use of the scale to
  `holder_id`; returns 409 with `{"error", "held_by_id", "held_by_name"}` if
  another vendor already holds it. Re-claiming with the same `holder_id`
  always succeeds and renews the claim.
- `POST /scales/{id}/release` — body `{"holder_id": "vendor-1"}`. Gives up
  `holder_id`'s claim, if it still holds one. Always returns
  `{"released": bool}` — releasing is a best-effort cleanup action, so a
  stale or already-lost claim is a silent no-op rather than an error.

### Scale claims: one vendor per scale at a time

Two vendors' phones can reach the same scale-gateway (it's shared across
one stall's local network), so nothing before this stopped two of them from
sending prices to the same physical scale at once. A claim is a
lightweight, in-memory reservation — held per scale, keyed by an opaque
`holder_id` the caller chooses (the mobile app uses the vendor's core-api
user id) — that makes that exclusive.

A claim expires on its own after `claimTTL` (20s) if never renewed, so a
crashed app or a phone that drops off the network doesn't permanently
strand a scale — see the mobile app's README for how it renews claims
while in active use and releases them proactively (added to receipt,
on-screen inactivity, the screen locking) well within that window. The
20s server-side TTL is deliberately just a backstop, not the primary
release mechanism.

This is a courtesy mechanism, not authentication: like the rest of this
service, it trusts callers on the local network to send a truthful
`holder_id`. It stops two vendors from *accidentally* colliding on one
scale; it isn't a security boundary.

This is a first-draft API surface designed to unblock the mobile app; it
will evolve once the exact mobile app flows are finalized.

## Running

```
go run ./cmd/scale-gateway -config config.example.json
```

## Testing locally without a physical scale

`cmd/fake-scale` simulates a Dialog-speaking scale over a real TCP
listener: it answers every set-price request with a fixed weight and the
total that implies, exactly like a real scale after settling. Point a
scale-gateway config's `address` at it instead of real hardware:

```
go run ./cmd/fake-scale -addr :9999 -variant 02 -weight-grams 1250
```

then, in another terminal, run scale-gateway against a config whose scale
`address` is `127.0.0.1:9999`. `internal/fakescale`'s own tests drive the
*real* `driver.DialogTCPDriver` (the same code used against actual
hardware) against this simulator — verified against production client
code, not just its own codec — and this was additionally run as real,
separate OS processes (fake-scale + scale-gateway binaries, talking over
actual TCP sockets, driven by real HTTP requests) to confirm the whole
pipeline end to end, not just via Go's in-process tests.

Flags: `-variant` (`02` or `04`), `-weight-grams` (default `1250`),
`-status` (defaults to `"1"` for variant 02, `"100"` for variant 04) — see
the note above on the status field's meaning being unknown regardless of
which value you pick.

## Testing

```
go test ./...
```

Driver tests use `net.Pipe` to simulate a scale without any real network
dependency; gateway/server tests use a fake `ScaleDriver` for the same
reason. No test depends on real hardware.
