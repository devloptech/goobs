---
name: test
description: Run full test suite for the goobs project with verbose output and coverage report
disable-model-invocation: true
user-invocable: true
allowed-tools: Bash
---

Run the full test suite for goobs with coverage:

```bash
go test ./... -v -count=1 -coverprofile=coverage.out && go tool cover -func=coverage.out
```

Report a summary:
- Total tests: PASS/FAIL count
- Coverage percentage per function
- Any failures with file:line references

If tests fail, read the failing test and source file to diagnose the issue.
