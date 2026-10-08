## Why

General browser QA of the current manager confirmed ten reproducible regressions: numeric quotas cannot be saved, webhook labels toggle a different event, login errors expose validator internals, unnamed controls block accessible navigation, light semantic and muted colors fail contrast, and mobile section labels collapse.

## What Changes

- Accept actual numeric input values while preserving omitted quotas and explicit unlimited zero.
- Show actionable required-field login messages.
- Give each webhook event its own label target.
- Hide empty column-display controls while retaining account column options.
- Keep native row semantics around instance links, selection checkboxes and actions.
- Name icon navigation, file pickers and the confirmed unlabeled selects; identify selected image thumbnails with alternative text.
- Restore semantic text contrast and readable mobile instance navigation.
- Repeat the frontend quality gates and manual browser QA with before/after evidence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `wzap-manager`: accessible manager controls and form validation at supported screen sizes.

## Impact

Nuxt manager components, shared quota parsing, locale copy and existing semantic CSS tokens; no REST/event contract or dependency changes.

## Out-of-Scope

Backend changes, gateway deployment, real WhatsApp deliveries, production record mutations and a visual redesign.
