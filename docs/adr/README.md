# Architecture Decision Records

This directory is the authoritative index for architectural decisions that still govern AnchorScan. Each record is immutable after acceptance; a later decision supersedes it rather than rewriting history.

| Status | Decision |
| --- | --- |
| Accepted | [ADR-0001: Defer scan intake unification](0001-defer-scan-intake-unification.md) |
| Accepted | [ADR-0002: Use SQLite Run Lease for one active run](0002-use-sqlite-run-lease.md) |
| Accepted | [ADR-0003: Persist DetectionCheck execution facts](0003-persist-detection-check-facts.md) |
| Accepted | [ADR-0004: Model Project as a penetration-test engagement](0004-model-project-as-pentest-engagement.md) |
| Accepted | [ADR-0005: Render DOCX through docxtpl](0005-docx-rendering-via-docxtpl.md) |

## Historical records

| Status | Decision |
| --- | --- |
| Rolled back | [ADR-0004: Identify built-in DetectionCheck candidates](archive/0004-identify-builtin-detection-checks.md) |
| Superseded by ADR-0006 | [ADR-0005: Exclude BlueKeep from default built-in probes](archive/0005-exclude-bluekeep-from-default-builtin-probes.md) |
| Superseded by ADR-0007 | [ADR-0006: Adopt Rapid7 BlueKeep baseline](archive/0006-adopt-rapid7-bluekeep-scan-baseline.md) |
| Historical accepted | [ADR-0007: Defer default built-in probes pending a safe candidate](archive/0007-defer-default-builtin-probe-until-safe-candidate.md) |

Historical records are retained for traceability and do not govern the current design unless a current ADR explicitly adopts them. New ADRs use the next unused number across this directory and `archive/`; the next available number is ADR-0008.
