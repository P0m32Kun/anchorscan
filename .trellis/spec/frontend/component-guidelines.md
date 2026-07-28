# Component Guidelines

> How components are built in this project.

---

## Overview

<!--
Document your project's component conventions here.

Questions to answer:
- What component patterns do you use?
- How are props defined?
- How do you handle composition?
- What accessibility standards apply?
-->

(To be filled by the team)

---

## Component Structure

<!-- Standard structure of a component file -->

(To be filled by the team)

---

## Props Conventions

<!-- How props should be defined and typed -->

(To be filled by the team)

---

## Styling Patterns

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

(To be filled by the team)

---

## Accessibility

<!-- A11y requirements and patterns -->

(To be filled by the team)

---

## Workbench Verification Dialogs

### Scope

The workbench has two verification dialogs that must behave symmetrically: the positive/confirm dialog and the negative/not_observed dialog. Both open from the candidate queue and share the same save/display pattern.

### Contracts

- Both dialogs look up the existing verification by `(zone_id, vulnerability_key)` on open.
- If a verification exists, the dialog loads its current `Title`, `Severity`, `Description`, and `Evidence` list into local state.
- The dialog renders already-uploaded evidence with a caption (`已上传：caption` or `无说明`).
- On save:
  - New verification: create with assets/sources, then upload pending files, then update metadata.
  - Existing verification: upload any new pending files, then update metadata (title/severity/description) so edits persist without requiring a new upload.
- The create payload includes `assets` and `sources`; the update payload must not include them.
- Both use the public snake_case contract (`zone_id`, `vulnerability_key`, etc.) and the shared `postJSON` helper.

### Validation & Error Matrix

| Condition | Required behavior |
|---|---|
| No verification and no pending files | Block save: "请至少上传一张截图作为证据". |
| Existing verification and no new pending files | Allow save to update metadata only. |
| Update `not_observed` without evidence | HTTP 400 from the backend; the dialog must only reach this state if evidence already exists or was just uploaded. |

### Good vs Bad

#### Bad

```ts
function openNegativeDialog(group) {
  resetNegativeDialog();
  negTitle.value = group.Title;
  negativeDialog.value?.showModal(); // existing evidence is not loaded
}
```

#### Good

```ts
async function openNegativeDialog(group) {
  resetNegativeDialog();
  negTitle.value = group.Title;
  const v = verificationMap.value[verificationKey(group.ZoneID, buildNegativeKey(group.ZoneID, group.Title))];
  if (v?.ID) {
    negVerificationId.value = v.ID;
    const detail = await getJSON(`/projects/${projectID}/verifications/${v.ID}`, ...);
    negUploadedEvidence.value = detail.Evidence;
    negSeverity.value = detail.Verification.Severity;
    negDescription.value = detail.Verification.Description;
  }
  negativeDialog.value?.showModal();
}
```

### Tests Required

- Frontend type-check: both dialogs import and use `postJSON`, `getJSON`, `normalizeVerificationDetail`, and `normalizeSeverity`.
- Web browser smoke: create a verification, upload evidence, reopen the dialog, and assert the existing evidence image is visible.
