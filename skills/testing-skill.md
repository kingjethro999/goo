---
name: testing
description: Use this skill whenever Goo is asked to write tests, add test coverage, set up a testing framework, or verify a change didn't break anything. Covers unit/integration/e2e strategy across Go and JS/TS codebases. Trigger on "write tests", "add test coverage", "make sure this works", or any _test.go / .test.ts / .spec.ts file.
---

# Testing Skill

## When to use
Any task asking for tests, coverage, or verification that a change is safe.

## Test pyramid — default effort allocation
- Most tests: unit tests on pure functions and business logic (fast, cheap, run on every save).
- Some tests: integration tests against a real test database or a real subprocess, for the seams between components.
- Few tests: end-to-end tests through the actual UI or CLI, for the handful of critical user flows.

Don't invert this. A suite that's mostly slow e2e tests with no unit coverage is expensive to run and slow to tell you what broke.

## Go testing pattern (table-driven, the idiomatic default)
```go
func TestValidateEmail(t *testing.T) {
    cases := []struct {
        name  string
        input string
        want  bool
    }{
        {"valid", "user@example.com", true},
        {"missing at", "userexample.com", false},
        {"empty", "", false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := ValidateEmail(tc.input)
            if got != tc.want {
                t.Errorf("ValidateEmail(%q) = %v, want %v", tc.input, got, tc.want)
            }
        })
    }
}
```

## JS/TS testing pattern (Vitest/Jest)
```ts
import { describe, it, expect } from "vitest";
import { calculateProfit } from "./calculateProfit";

describe("calculateProfit", () => {
  it("returns zero when entry equals exit price", () => {
    expect(calculateProfit({ entry: 100, exit: 100, size: 10 })).toBe(0);
  });

  it("handles a losing trade", () => {
    expect(calculateProfit({ entry: 100, exit: 90, size: 10 })).toBe(-100);
  });
});
```

## What to actually test (priority order)
1. Business logic with branching (pricing, risk calculations, permission checks) — highest bug density, highest cost if wrong.
2. Anything touching money, auth, or user data deletion.
3. Edge cases: empty input, zero, negative numbers, very large input, unicode in strings.
4. The specific bug just fixed — add a regression test in the same commit as the fix, not "later."

## What not to bother testing
- Simple prop-passthrough React components with no logic
- Third-party library internals (trust the library's own tests)
- Getters/setters with no logic

## Mocking guidance
Mock at the boundary (the Groq client, the database driver, the filesystem), not the function under test itself. If a test needs more than two or three mocks to run, that's usually a sign the function under test is doing too much and should be split.

## CI checklist
- [ ] Tests run on every PR, not just locally before merge
- [ ] A failing test blocks merge — no "fix it in the next PR"
- [ ] Test database/fixtures are isolated per run — no shared mutable state between parallel test runs
- [ ] Coverage is a signal, not a target — 100% coverage with no real assertions is worse than 70% that actually checks outcomes

## Common pitfalls to avoid
- Asserting on implementation details (internal state, private fields) instead of observable behavior
- Tests that depend on execution order
- Fixed-delay sleeps instead of waiting on a real condition (flaky under load)
- Writing the test after being told "just make it pass" instead of writing it to reflect the actual spec
