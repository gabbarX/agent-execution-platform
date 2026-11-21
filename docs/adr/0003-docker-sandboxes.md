# 0003. Docker containers as the sandbox boundary

**Status:** accepted
**Date:** 2026-11-21

## Context

The reference architecture recommends microVMs (Firecracker) for running
untrusted, model-generated code: each run gets its own guest kernel.
Firecracker is Linux-only and does not run natively on this Windows 11
development machine; gVisor likewise. Docker Desktop is available.

## Decision

One Docker container per run is the isolation boundary. Container
lifecycle (one run per sandbox, destroyed after use, warm pools, egress
lockdown) follows the reference design exactly.

## Consequences

The sandbox *lifecycle* mechanics — the part this project exists to
demonstrate — are faithful to the reference. The isolation *boundary* is
weaker: containers share the host kernel, so a kernel escape compromises
neighbors; production would use microVMs. This trade-off is stated, not
hidden — interview answers should lead with it. Revisit: if the project
moves to a Linux host, swapping the container runtime for
Firecracker-via-containerd (Kata/firecracker-containerd) is contained in
the sandbox manager.
