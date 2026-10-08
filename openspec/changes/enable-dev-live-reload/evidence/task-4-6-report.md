# Task 4 + Task 6 implementation

Scope: manager/nuxt.config.ts, docker-compose.dev.yml, tasks 2.1/4.1/4.2 and task-4-* / task-4-6-* evidence. Base04e1af98837a2891e00f5f41fa853c61448b709f. No application, Air, dependencies, production Dockerfile, migrations, CI or unrelated AGENTS changes. No integration-test suite is claimed. No further delegation.

## Changes and local source evidence

- manager/nuxt.config.ts:68 development-only hooks; :75 client-only Vite runtime ws.clientPort from NUXT_DEV_HMR_CLIENT_PORT. SSR false, app /manager/, same-origin API and existing standalone Vite proxy preserved.
- Initial direct ws option produced typecheck RED (task-4-nuxt-typecheck-red.log). Installed @nuxt/schema4.5.2 imports Vite at dist/index.d.mts:40 and references ServerOptions at :556. Its undeclared Vite resolves through .pnpm/node_modules/vite to Vite7.3.6, whose server.ws type is only false (:2368). @nuxt/vite-builder4.5.2 resolves Vite8.2.1, whose WsOptions.clientPort is dist/node/index.d.ts:1173 and server.ws accepts WsOptions at :2544. Development vite:extendConfig merges the runtime server options via Object.assign without changing dependencies or coercing schema types. Final typecheck GREEN exit0.
- manager/nuxt.config.ts:72 font public-asset-context hook corrects development-only CSS URLs to /manager/_fonts. @nuxt/fonts0.14.0 dist/module.mjs:105 defaults CSS asset URL to /_fonts, :107 calls public-asset-context at modules:done, :124 registers its actual dev handler with app.baseURL joined already. Thus prefixing configuration directly would duplicate the route; updating the public asset context after handler registration aligns generated URLs with the existing route. Before: /_fonts/*.woff2 404 through Go; after /manager/_fonts/*.woff2 200. Production hook is inactive.
- docker-compose.dev.yml:4 replaces same wzap service/image with wzap:dev-live-reload; :10 Go manager upstream; :15 manager dependency without a profile; :19 inherited health timing with dev binary; :20 20s stop grace. :22 internal Node24 manager uses locked pinned pnpm and separate dependency/pnpm-store/.nuxt/.output volumes; no host port. Base project/network/8081/media/infrastructure retained.

## Commands and acceptance

All commands below ran from /home/obsidian/dev/wzap/.worktrees/enable-dev-live-reload. with-runtime-env.py injects preserved credentials without printing them. Compose JSON containing environment was kept private and never staged; task-4-6-compose.json is a sanitized service/mount/port/health/dependency projection.

```
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.dev.yml config --format json
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.dev.yml config --format json
# PASS project wzap; one Go service wzap; public127.0.0.1:8081:8080; manager no host ports; inherited media/data/dependencies/health timing.
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.dev.yml up -d --no-deps wzap manager-dev
# Dev image already built during Task5; no infrastructure recreated. Initial locked install cold boot3m17s; cold Go build~3min.
pnpm --dir manager typecheck
# initial direct config RED exit2; runtime hook GREEN exit0; font-hook final typecheck GREEN exit0.
agent-browser --session wzap-dev-live-reload-t46 --init-script .superpowers/sdd/plan/browser-hmr-init.js open about:blank
agent-browser --session wzap-dev-live-reload-t46 open http://127.0.0.1:8081/manager/login
agent-browser --session wzap-dev-live-reload-t46 auth login wzap-live-reload-qa
agent-browser --session wzap-dev-live-reload-t46 wait --text 'Recent instances'
# Disposable unprivileged vault login successful; no passwords/tokens or private credential JSON printed/staged.
```

Automatic Go proof temporarily changed health.go status ok to live-reload-proof. Corrected helper observed wrapped data.status after36.7s, restored source in finally, then observed stable ok on subsequent production/current dev probes. First helper wrongly expected an unwrapped status, timed out despite separately observed changed response, and source was immediately restored; this harness error is not claimed as application failure. Air handled actual Go edit/restore without manual restart.

Browser initial blank/module/navigation states crossed temporary Go reload and Nuxt configuration restart; a stable reopen rendered login and authenticated overview. Acceptance uses the stable rendering, not those earlier transient pages. Temporary OverviewHeader.vue title Live Vue proof and CSS h1 magenta automatically appeared without navigation/reload; socket receive count19→38, no internal-port fallback. Desktop1280x800 and mobile390x844 screenshots show the title/color and responsive layout and were opened with view_image. Restoration automatically returned title Overview and normal color; final desktop restored screenshot inspected. A standard-library socket probe privately extracts Vite's client token and sends a real upgrade request; saved result is HTTP/1.1 101 Switching Protocols on ws://127.0.0.1:8081/manager/_nuxt/, query omitted. Browser socket evidence likewise records only origin/path, lifecycle and counts. Modules and fonts are /manager/_nuxt/... and /manager/_fonts/... with no doubled /manager prefix. All proof source files restored byte-for-byte, no diagnostic UI committed.

Snapshots compare running IDs, mounts, port mappings and counts+hashed IDs against runtime-before.json and data-before-with-qa.json. No legacy wzap-dev Go service existed. Postgres15fef8eb…, NATS73634d67…, Miniod8168a90… stay identical throughout, with wzap_pgdata/wzap_natsdata/wzap_miniodata/wzap_media retained. Go and manager container IDs remain identical across Go/Vue/CSS source edits. Data remains2users/1instance, identical identity fingerprints (includes controller's disposable QA).

```
python3 .superpowers/sdd/plan/task-4-6-snapshot.py
# After dev activation, Go edit/restore, Vue/CSS edits, stopped state, compiled state, final dev activation: assert same infrastructure+data.
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.dev.yml stop wzap manager-dev
# Snapshot asserts both exited BEFORE compiled startup.
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml up -d --no-deps wzap
# compiled wzap:dev running; Nuxt exited; only inherited media volume; original image ID unchanged. HTTP200 /healthz, /manager/, /manager/instances. Data/infra equality PASS.
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml stop wzap
python3 .superpowers/sdd/plan/with-runtime-env.py docker compose -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.dev.yml up -d --no-deps wzap manager-dev
# return to dev for Task7; no second Go container; infrastructure unchanged.
git diff --check
# PASS; source proof files have zero git diff.
```

## Boundaries / remaining work

Task7 owns broad Go/manager gates, additional navigation/refresh/reconnection validation, Nuxt503 recovery, documentation and a fresh production build from a sanitized context. This task proves the real switch to the existing compiled wzap:dev image; it does not claim a fresh build of the current production code. No production build ran here, so no scratch credential files were transmitted into layers. The controller must exclude .superpowers, .worktrees, .git, local .env and current ignored frontend outputs/dependencies from that build context.

Runtime observation: Go/manager startup readiness is asynchronous (manager dependency is service_started); cold start produces temporary503 until Nuxt is ready. Production switch emits normal Compose orphan warning for the stopped manager container; no --remove-orphans/down -v used. Cache-volume creation warned that existing Go caches were not created by Compose; those caches contain no data volumes and were retained.

Task evidence in openspec/changes/enable-dev-live-reload/evidence/: task-4-nuxt-typecheck-red.log, task-4-nuxt-typecheck-green.log, task-4-6-compose.json, task-4-6-go-proof.log, task-4-6-upgrade.json, task-4-6-hmr.json, task-4-6-runtime.json and three screenshots task-4-6-vue-css-hmr.png, task-4-6-mobile-hmr.png, task-4-6-desktop-restored.png. Private scratch snapshots remain under .superpowers/sdd/plan. No controller task-5 evidence staged by this implementer.

## Final cache isolation

pnpm11 created its default /src/manager/.pnpm-store in the source bind mount during cold install. Added manager-pnpm-store volume at the same path, stopped/recreated manager only, copied the task-created content into the new cache volume using a disposable Node container, and removed only that source store. Existing node_modules metadata retains the same store path; the locked install restarts without a path-change prompt. Initial `compose create --no-deps manager-dev` rejected the unsupported flag before mutation; retried supported `compose create manager-dev` (manager has no dependencies). Runtime evidence records the manager container recreation as a configuration change, distinct from the proven unchanged IDs during source edits.

Final cache install completed noninteractively in2.5s using pnpm11.22.0, Nuxt built successfully and /manager/200. Fresh browser login succeeded; temporary title Cache-isolated HMR proof appeared automatically over the public socket and restored to Overview. Final cache HMR edit/restore preserved current Go+manager IDs and infra/data equality; sources byte-identical again. task-4-6-final-cache-hmr.log records public socket open/counts and title. Final dev /healthz statusok and /manager/200; dev remains running.
