---
name: test-watch
description: Run a specific test function or test file for rapid iteration
argument-hint: "[TestFuncName or filename]"
disable-model-invocation: true
user-invocable: true
allowed-tools: Bash, Read
---

Run a specific test for rapid iteration:

If `$ARGUMENTS` looks like a test function name (starts with Test):
```bash
go test ./... -v -count=1 -run "$ARGUMENTS"
```

If `$ARGUMENTS` looks like a filename:
```bash
go test -v -count=1 -run "" ./$ARGUMENTS
```

If no argument provided, run all tests:
```bash
go test ./... -v -count=1
```

If the test fails, read the relevant source and test files to diagnose.
