---
name: add-builder
description: Scaffold a new fluent builder subsystem following goobs conventions
argument-hint: "[builder-name]"
disable-model-invocation: true
user-invocable: true
allowed-tools: Read, Write, Edit, Glob
---

Create a new fluent builder subsystem named `$ARGUMENTS` in the goobs package. Follow the established patterns exactly.

## Required Structure

1. Create `$0.go` in the project root with `package goobs`

2. Define the builder struct:
```go
type <Name>Obs struct {
    ctx   context.Context
    // subsystem-specific fields
}
```

3. Package-level constructor:
```go
func <Name>() *<Name>Obs {
    return &<Name>Obs{
        ctx: context.Background(),
    }
}
```

4. Required chainable methods:
   - `FromContext(ctx context.Context)` — nil-check ctx before assigning
   - `Attr(key string, val any)` — type switch: string, int, int64, float64, bool, default

5. Terminal method (e.g., `Send()`, `Execute()`, `Record()`) that:
   - Checks global state is initialized before proceeding
   - Attaches trace/span IDs from context when available
   - Handles both the OTel path and any fallback path

## Conventions from Existing Code

Before writing, read these files to match the exact style:
- `logger.go` — for dual-write pattern and caller resolution
- `trace.go` — for builder with multiple terminal methods
- `metric.go` — for lazy-create with mutex-protected cache
- `config.go` — add any new config fields needed

## Checklist
- [ ] Follows `package goobs` (single package, no subdirectories)
- [ ] Uses global vars for providers/instruments
- [ ] No-ops gracefully when not initialized
- [ ] Type switch in Attr matches: string, int, int64, float64, bool, default
- [ ] FromContext nil-checks the context
