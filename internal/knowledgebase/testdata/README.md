# Catalog v2 test fixtures

## Reduced fixture

- Protocol: `version: 2`, `source: handbook-v3`
- `catalog-v2.json` is a committed reduced fixture.
- `handbook-v2.md` is the matching legacy Markdown projection for its shared `smb-signing` subset. Its anchorscan-catalog marker remains version 1 because Markdown compatibility is legacy.

## Frozen producer artifact

- Source repository: `Pentest-Playbook`
- Source artifact: `handbook-v3/dist/catalog.json`
- Source commit: `57d739e`
- Protocol: `version: 2`, `source: handbook-v3`
- SHA-256: `7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e`
- Acquired: `2026-08-05`

`catalog-v2-real.json` is a read-only committed copy for drift detection. Tests never read a neighboring Pentest-Playbook worktree at runtime.
