# Testing Strategy

This document defines how AnchorScan chooses automated test seams. Operational steps for the real-tool laboratory live in [testing-lab-checklist.md](./testing-lab-checklist.md).

## Principle: Use the Lowest Sufficient Seam

Test each behavior at the lowest layer that can reproduce the actual risk through a public boundary. Do not duplicate the same assertion across layers by default. Add a higher-level test only when a lower-level test cannot verify browser behavior, process wiring, packaging, or collaboration with real external tools.

| Behavior under test | Preferred seam |
| --- | --- |
| Pure business rules, filtering, sorting, aggregation, and error classification | Go or JavaScript unit test |
| HTTP status, response body, and stable error-code mapping | Go `httptest` handler test |
| SQLite persistence, transactions, migrations, and cleanup | Store or integration test |
| Browser navigation, forms, focus, dialogs, rendering, clipboard, and downloads | Playwright browser smoke |
| Real rustscan, nmap, httpx, and nuclei collaboration | Build-tagged Docker laboratory E2E |
| Release archive contents and DOCX sidecar wiring | Packaging or focused integration test |

## Browser Coverage

Use Playwright for representative, user-visible workflows where a real browser is part of the contract. Browser tests should exercise the production HTTP entrypoint with isolated configuration, a temporary SQLite database, and deterministic scanner fixtures. Do not add production-only test endpoints or frontend test modes.

Playwright is appropriate when a change affects:

- navigation or form submission;
- keyboard access, focus restoration, dialogs, or responsive rendering;
- browser APIs such as clipboard, file upload, or downloads;
- the wiring between a user action, an HTTP response, and rendered feedback.

Playwright is not required for exhaustive error matrices, pure report construction, Store failure branches, or scanner routing already verified at a lower seam. A representative browser success or failure path is enough when lower-level tests cover the remaining combinations.

Browser failures must retain actionable diagnostics: screenshot, trace, console errors, and relevant server output.

## Real-Tool E2E Coverage

The Docker laboratory verifies behavior that deterministic browser fixtures cannot:

- execution of real scanner binaries;
- routing across web and non-web service families;
- persistence of partial facts when an optional tool fails;
- CLI/Web orchestration against reachable container targets.

Real-tool E2E is separate from Playwright and is not required on every PR. Release evidence must identify the commit, date, tool versions, and test result.

## Stable Commands

```bash
make test      # Go and JavaScript tests
make harness-check # AI workflow contracts; no network or runtime-session state
make pr-check  # Unit/integration checks, build/package, and Playwright Chromium smoke
make e2e       # Real-tool Docker laboratory
```

The Make targets are the canonical interface for local runs, documentation, and CI. Update the target first when the underlying command changes.

## Change Checklist

Before completing a change:

- [ ] Is the test at the lowest seam that can reproduce the risk?
- [ ] If Playwright was added, does the assertion require a real browser?
- [ ] If real-tool E2E was added, does it require Docker or an external scanner binary?
- [ ] Does each test layer catch a distinct failure rather than repeat the same assertion?
- [ ] Will a failure leave enough diagnostics to identify the broken boundary?
