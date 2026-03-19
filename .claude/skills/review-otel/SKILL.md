---
name: review-otel
description: Review recent changes for OpenTelemetry correctness and goobs pattern compliance
disable-model-invocation: true
user-invocable: true
allowed-tools: Bash, Read, Grep, Glob
---

Review the current code changes for OTel correctness and goobs pattern compliance.

## Steps

1. Run `git diff` to see current changes (staged + unstaged)
2. For each changed file, verify:

### Builder Pattern
- Constructor returns pointer to builder struct
- All chainable methods return `*<Type>` for fluency
- `FromContext` nil-checks ctx
- `Attr(key string, val any)` uses type switch with: string, int, int64, float64, bool, default

### OTel Correctness
- Spans are always ended (`defer span.End()` or `Done()`)
- Errors recorded on spans: `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)`
- Metrics gated on `Config.EnableMetrics` and `globalMeter != nil`
- Instruments cached with mutex protection

### Context Propagation
- Extract on inbound, Inject on outbound
- Carrier satisfies `propagation.TextMapCarrier` interface (Get, Set, Keys)
- Legacy headers only with `WithLegacyHeaders(true)`

### Logging
- Dual-write: OTel log + Zap
- Trace/span IDs attached from context
- Caller resolution respects SkipCallerPkgs/SkipCallerFiles

3. Report findings as: Critical / Warning / Suggestion with file:line references
