---
name: check
description: Run build, vet, and tests for the goobs project
disable-model-invocation: true
user-invocable: true
allowed-tools: Bash
---

Run the full verification pipeline for goobs:

```bash
go build ./... && echo "BUILD: PASS" || echo "BUILD: FAIL"
```

```bash
go vet ./... && echo "VET: PASS" || echo "VET: FAIL"
```

```bash
go test ./... -count=1 && echo "TEST: PASS" || echo "TEST: FAIL"
```

Report a one-line summary per step. If anything fails, show the relevant error output.
