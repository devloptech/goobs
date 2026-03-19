# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`goobs` is a Go library (`github.com/devloptech/goobs`) that provides a unified observability facade over OpenTelemetry and Zap. It wraps tracing, logging, metrics, and context propagation behind fluent builder APIs so consuming services don't interact with OTel SDKs directly.

## Build & Test

```bash
go build ./...
go test ./...
go vet ./...
```

No Makefile or custom tooling — standard Go commands only. Requires Go 1.25.5+.

## Architecture

Single-package library (`package goobs`) with global state initialized via `Init()`. All subsystems share a single OTLP/gRPC endpoint.

### Init (`goobs.go`)
`Init(ctx, Config)` sets up all providers (trace, metric, log) and returns a `shutdown` function. Global state is protected by `sync.RWMutex` — `Init()` takes a write lock, all readers use `getGlobals()` with a read lock. Calling `Init()` again safely shuts down previous providers before re-initializing.

### Subsystem builders — all use fluent/chainable API pattern:

- **`Log()` → `LogObs`** (`logger.go`): Dual-writes to both OTel log exporter and Zap. Auto-attaches trace/span IDs and caller info. Caller resolution walks the stack and skips frames matching `Config.SkipCallerPkgs`/`SkipCallerFiles`.

- **`Trace()` → `TraceObs`** (`trace.go`): Wraps OTel span creation. Three usage modes: `Start()` returns raw ctx+span, `StartScope()` returns a `SpanScope` with `Done()`, `Run(fn)` executes a function within a span and auto-records errors.

- **`MetricCounter()` / `MetricHistogram()` / `MetricGauge()`** (`metric.go`): Lazy-create and cache OTel instruments by `name|unit|desc` key. No-ops when `Config.EnableMetrics` is false.

- **`Propagate()` → `PropagationObs`** (`http.go`): Context propagation inject/extract for HTTP, gRPC metadata, and AMQP headers. Optional legacy `x-trace-id`/`x-span-id` headers via `WithLegacyHeaders(true)`.

- **`AMQPConsumerInterceptor()`** (`amqp.go`): Pre-built interceptor that wraps an AMQP consumer handler with tracing, logging, and metric collection.

## Key Patterns

- All builders start from a package-level constructor (`Log()`, `Trace()`, `Propagate()`, etc.) and chain methods before a terminal call (`Send()`, `Start()`, `Run()`, `Add()`, `Record()`).
- Metrics are gated on `Config.EnableMetrics`; trace and log are always enabled after `Init()`.
- OTel instruments are cached in package-level maps with mutex protection (`metric.go`).

## Custom Agents

Located in `.claude/agents/`. Claude delegates to these automatically based on task context, or you can invoke them with `@"agent-name"`.

| Agent | Purpose |
|---|---|
| `otel-reviewer` | Reviews code for correct OTel patterns, builder API consistency, and goobs conventions. Use after writing/modifying observability code. |
| `test-runner` | Runs `go build`, `go vet`, and `go test` pipeline and reports results. Use after code changes. |
| `test-writer` | Writes comprehensive Go tests following goobs conventions (in-memory exporters, table-driven tests, builder chain verification). |
| `propagation-debugger` | Diagnoses context propagation issues across HTTP, gRPC, and AMQP transports. |

## Custom Skills (Slash Commands)

Located in `.claude/skills/`. Invoke with `/<skill-name>`.

| Skill | Usage | Purpose |
|---|---|---|
| `/add-builder` | `/add-builder <name>` | Scaffolds a new fluent builder subsystem following goobs conventions (struct, constructor, chainable methods, terminal method). |
| `/check` | `/check` | Runs the full build + vet + test pipeline with pass/fail summary. |
| `/review-otel` | `/review-otel` | Reviews current `git diff` for OTel correctness and goobs pattern compliance. |
| `/test` | `/test` | Runs full test suite with verbose output and coverage report. |
| `/test-watch` | `/test-watch [TestName]` | Runs a specific test function or all tests for rapid iteration. |

## Testing

Tests use in-memory OTel exporters via `setupTestEnv()` in `testing_helper_test.go`. This avoids needing a real OTLP collector. Each test file corresponds to its source:

| Source | Test | Coverage area |
|---|---|---|
| `goobs.go` | `goobs_test.go` | Init, shutdown, globals, idempotency |
| `logger.go` | `logger_test.go` | Builder chain, field types, Err(), Send(), caller resolution, severity mapping |
| `trace.go` | `trace_test.go` | Builder chain, Start/StartScope/Run, error recording, span attributes |
| `metric.go` | `metric_test.go` | Counter/Histogram/Gauge, caching by name\|unit\|desc, no-op when disabled |
| `http.go` | `http_test.go` | HTTP/gRPC propagation inject/extract, carrier implementations, legacy headers |
| `amqp.go` | `amqp_test.go` | AMQP round-trip propagation, interceptor span/metrics, context propagation |
