# manager

Nuxt 4 console (`baseURL /manager/`, EN, SSR off), embedded in the Go binary.
Root [AGENTS.md](../AGENTS.md) still applies; this file wins under `manager/`.

## Setup commands

- `pnpm --dir manager install` · `pnpm --dir manager dev` · `pnpm --dir manager build` (`nuxt generate` → `.output/public`)
- Air + Nuxt: `docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build wzap manager-dev` from repo root; insert local override between base/dev when present. Panel/HMR use `:8081/manager/`; standalone proxy defaults to `http://127.0.0.1:8081`.
- Embed is `go:embed all:.output/public` (`all:` is required for `_nuxt/`). `WZAP_MANAGER_DIR` serves disk, else embed; unbuilt → 503.

## Testing

`pnpm --dir manager test` (Vitest) · `lint` · `typecheck`. CI `manager` job only
installs, builds, and checks `index.html` — still run the three locally.

## Code style

Node 24 + pnpm (no npm/yarn lockfiles). ESLint: no trailing commas, `1tbs`.
Copy in `i18n/locales/en.json`. Same-origin API, cookie `wzap_session`, no CORS.

## Boundaries

- Always: verify the changed UI flow; instance `{id}` is UUID or exact name.
- Ask first: new Nuxt modules or prod frontend deps.
- Never: change the REST contract only in the console; commit `.output/` except the embed placeholder.
