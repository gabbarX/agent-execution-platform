# Architecture (Current State)

> This document describes what **exists right now**. It grows with each
> milestone. The vision lives in the design doc
> (`docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`);
> the reference architecture lives in `docs/REFERENCE.md`.

**As of:** M0 (repo foundation)

## Module Layout

```
agent-execution-platform/
├── internal/            private platform packages
│   └── version/         platform version string (the toolchain proof)
├── scripts/
│   └── check.ps1        the quality gate: fmt → vet → lint → test → build
├── docs/                all documentation (see README.md for the map)
├── demos/               per-milestone demo scripts
│
│   — planned, not yet created —
├── cmd/                 binaries: controlplane, worker, example agents
├── sdk/                 public Go SDK agents are written against
├── agents/              example agents
└── deploy/              docker-compose for Postgres etc.
```

## The Quality Gate

`pwsh -File scripts/check.ps1` runs gofmt (fail on any unformatted file),
`go vet`, `golangci-lint`, `go test ./...`, `go build ./...` and exits
non-zero at the first failure. Every commit must pass it.

## Decisions

Recorded as ADRs in `docs/adr/`. Current: 0001 (Go), 0002 (vertical-slice
milestones), 0003 (Docker containers as the sandbox boundary).
