# 0002. Build in vertical slices, not components or on Temporal

**Status:** accepted
**Date:** 2026-11-21

## Context

Three ways to sequence the build: component-by-component in article order
(nothing runs end-to-end until most components exist); on top of Temporal
(realistic industry choice, but it outsources durable execution — the most
instructive subsystem); or vertical slices.

## Decision

Vertical slices: a thin end-to-end walking skeleton first (M1), then one
deep concern per milestone (durability, SDK/replay, sandboxes, gateways,
approvals, observability).

## Consequences

Every milestone ends in a demo and teaches one big idea; motivation and
learning stay high across a months-long project. Cost: some early code is
deliberately naive (in-memory queues, fake agent) and gets rewritten as
later milestones deepen it — accepted, the rewrite *is* the lesson.
Revisit: if the project ever needed production durability guarantees,
Temporal would be the honest choice.
