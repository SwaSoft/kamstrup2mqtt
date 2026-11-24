# 0001 — Package layout

Date: 2026-11-24 · Status: accepted

## Context

kamstrup2mqtt is a single Go module (`kamstrup2mqtt`) whose code lives in one `main` package at the repo root: `bridge.go` (Bridge, Metrics), `core.go` (BridgeLogic), `config.go` (Config, DefaultConfig, LoadConfig, LoadCredentials) and `parser.go` (FrameParser). Organisation-wide ADRs (swasoft 0001 Functional Core, Imperative Shell; 0002 Standard Repository Layout) ask for pure logic to be separated from side effects and for packages to be named after their responsibility, not after technologies. This component is small, so ADR 0002's separate `core`/shell *modules* would add ceremony; we use packages inside the single Go module instead.

## Decision

The Go module name stays `kamstrup2mqtt`. The layout is:

- `main.go` (repo root): the only entrypoint (`package main`). It parses CLI flags, wires the packages together and starts the application. It holds no domain logic.
- `internal/config`: the Config type, DefaultConfig, LoadConfig and LoadCredentials. The Config JSON schema is unchanged.
- `internal/parser`: FrameParser and serial frame parsing. Pure, with no I/O.
- `internal/bridge`: Bridge, Metrics and BridgeLogic. BridgeLogic holds the pure decision logic (functional core); Bridge performs the serial and MQTT I/O (imperative shell).

Packages are named after their responsibility. Dependencies point inward: `main` may import every `internal` package; `bridge` may import `config` and `parser`; `config` and `parser` import no other internal package. New packages go under `internal/` and are recorded in a new ADR.

The refactor preserves runtime behaviour, configuration compatibility, and the existing dependencies.

## Consequences

- Positive: clear boundaries between configuration, parsing and bridging; pure logic is testable without I/O; `internal/` prevents outside imports.
- Trade-offs: identifiers that tests access must be exported or tested from within their new package; files and tests move with their packages.

## Index

See [README.md](README.md) for the ADR index.
