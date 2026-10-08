## Context

Evidence is in `dogfood-output/frontend-2026-10-07/report.md` in the primary checkout. The current UI uses Nuxt UI v4 primitives. Its numeric input emits numbers; form-field injection assigns a shared id to all nested webhook checkboxes. The light theme uses 500 semantic shades.

## Goals / Non-Goals

Goals: restore intended form behavior and operability using existing components and copy.
Non-goals: replace the design system, change server contracts, or exercise real WhatsApp commands.

## Decisions

- Normalize string/number/empty quota inputs in the existing shared parser instead of coercing blanks to zero; use the same validation in creation and editing.
- Use per-component event ids with per-event suffixes, preserving the existing event ordering and PATCH payload.
- Apply accessible names directly to the primitive that receives focus, using existing locale labels.
- Adjust light semantic and muted CSS aliases without changing brand palettes or dark mode; contrast is checked on rendered elements rather than duplicated hex constants in unit tests.
- Use a mobile section selector with the full current label and a desktop navigation menu; this keeps every section discoverable without collapsed text or a second page-level scroll direction.
- Retain native table rows and use guarded pointer delegation against the displayed row model; native links and checkboxes remain keyboard targets.
- Override the exposed file-leading preview slot with a shared component using filename alternative text and VueUse object-URL cleanup; preserve grid sizing and file removal.
- Use an isolated worktree based on a hashed snapshot of the existing dirty manager, and copy back only guarded new deltas after verification.

## Risks / Trade-offs

[Nuxt UI forwards attributes to different roots] -> inspect installed primitive sources and verify real accessible names with agent-browser and axe.
[Light semantic changes affect many components] -> check solid/subtle/error/link/hover states and dark mode together.
[Running gateway implements the previous collection contract] -> use the documented read-only QA adapter and clearly label fixtures and integration limits.
[Existing work could be overwritten] -> compare each source file with its baseline hash before applying only the new delta.
