# Task 4 + Task 6 independent spec and quality review

Reviewed range: `04e1af98837a2891e00f5f41fa853c61448b709f..c44f024deb65d2cdc3c901f93d43ce998dfe8859`.
Worktree: `/home/obsidian/dev/wzap/.worktrees/enable-dev-live-reload`.

## Verdicts

- **Spec compliance: PASS for original OpenSpec 2.1/4.1/4.2.** The scoped implementation and retained observations support those checkboxes. This verdict does not mark the whole change or Task 7 complete.
- **Task quality: PASS.** No actionable P0/P1/P2/P3 finding identified in the reviewed scope. The dev-only runtime hook is a small, evidence-backed workaround for the installed Nuxt/Vite type mismatch; Compose preserves the established application entrypoint and infrastructure.

## Review inputs and method

Read the controller review brief, original implementation brief, final implementation report, and controller-generated diff package once. Did not regenerate git diff/log/show. Checked current scoped configuration against the change design/delta specs, resolved Compose evidence, runtime snapshots, HMR/upgrade/font evidence, final cache HMR log, Go proof and Nuxt typecheck records. Inspected all three retained screenshots using view_image. Read relevant installed Nuxt fonts/Vite builder/type definitions to assess hook timing and behavior. No tests, Docker actions, browser reruns, agents, staging, commits, or application edits performed. Only this review file was written.

## Spec acceptance evidence

### Nuxt and public HMR (2.1)

- `manager/nuxt.config.ts:16`, `:23`, `:33`, `:55` retain SSR false, `/manager/`, empty same-origin API base, and the standalone Vite API proxy. No Nitro API proxy is introduced.
- `manager/nuxt.config.ts:68-86` scopes both new hooks to development. The client-only condition at `:76` applies `NUXT_DEV_HMR_CLIENT_PORT` through runtime `server.ws.clientPort`; it preserves the other server settings and does not set a second manager base or HMR path.
- `evidence/task-4-6-upgrade.json:2-5` records a real HTTP 101 upgrade with vite-hmr on `ws://127.0.0.1:8081/manager/_nuxt/`, query omitted. `evidence/task-4-6-hmr.json:2-13` records the browser socket open and received messages at that same origin/path. Listed module assets stay beneath `/manager/_nuxt/`; no internal-port fallback is recorded.
- `evidence/task-4-6-hmr.json:69-74` records automatic Vue text and CSS changes. The desktop and mobile proof screenshots visibly show the temporary magenta title; the restored desktop screenshot shows Overview and normal color. Report `:33` records automatic edit/restoration without navigation or manual reload. This is actual browser evidence rather than config-only assurance.
- `manager/nuxt.config.ts:72-73` aligns dev font CSS URLs to `/manager/_fonts`. The installed fonts module registers its dev handler using app.baseURL before calling `fonts:public-asset-context` at modules:done; therefore the late context update changes emitted asset URLs without double-prefixing the registered handler. Retained font network entries record HTTP 200 beneath that public prefix. Production does not load the development hooks.

### Overlay and preserved switch (4.1/4.2)

- `docker-compose.dev.yml:3-20` overlays only the existing wzap application service. It inherits project `wzap`, the base public `127.0.0.1:8081:8080` port, media mount, infrastructure dependencies, and health timing. The resolved sanitized projection confirms these merges.
- `docker-compose.dev.yml:5-8` keeps `wzap:dev-live-reload` separate from compiled `wzap:dev` and selects Dockerfile.dev. `:19-20` uses `/src/tmp/wzap healthcheck` and a 20-second stop grace. No Air wrapper or application shutdown change appears in this package.
- `docker-compose.dev.yml:15-17`, `:22-34` makes manager-dev a normal required dependency, internally listening on 3000, with no published host port or profile. Node 24 and `pnpm install --frozen-lockfile` are configured; `manager/package.json` pins pnpm 11.22.0. Dependencies, pnpm store, .nuxt, and .output are separate named cache/output mounts.
- `evidence/task-4-6-runtime.json` dev-start, after-go, and after-hmr snapshots retain Go ID `227c095d9e45` and manager ID `d1a9405bc1f7`. The Go proof log records the changed wrapped health response after 36.7 seconds. The report distinguishes an initial proof-harness response-shape error from application behavior and records source restoration.
- Runtime snapshot stopped (`:476`) records both dev services exited before compiled (`:627`) records only the compiled Go service running and manager exited. Final-dev returns the same application service to the dev image. Every snapshot retains infrastructure container IDs, effective data-volume names, and identical counts/identity fingerprints: 2 users and 1 instance including the controller-owned disposable QA user. No legacy Go service existed according to the report; the package declares only one Go application service.
- Final-cache (`:928`) and final-cache-after-edit (`:1091`) retain Go ID `2913ba801f96` and manager ID `839c31acc91a`, with infra/data equality. The manager recreation from its earlier ID is explicitly a cache-mount configuration change, not a source edit. Final-cache HMR log records a new temporary title and open public socket receiving 32 messages after that recreation. Report `:61-63` records successful frozen noninteractive install, startup, login, automatic restoration, and HTTP readiness.

## Quality assessment

The workaround does not add dependencies or cast the stale Nuxt schema to a different Vite version. Installed schema imports Vite types while the installed builder uses Vite 8 runtime options; the retained RED error specifically rejects ws.clientPort as incompatible with the older false-only type, and GREEN/report records the hook version passing. The installed builder invokes vite:extendConfig before creating the client server, so the runtime mutation reaches the intended configuration. The font hook is likewise supported by installed module timing and observed 200 responses.

Compose's dev command leaves package-manager version resolution to the existing packageManager pin and enforces the lockfile. Service_started permits an expected startup window before Nuxt readiness; the report acknowledges temporary 503s. Cache volumes are isolated and the final evidence validates the configuration after the store mount was added. No production image/dependency/CI/migration/domain change is present in the supplied range. Evidence reviewed contains no HMR query token, cookie, password value, API credential, raw user/instance ID, or prohibited instance fields; user/instance identity comparison uses hashes. User-owned AGENTS changes and controller Task 5 evidence are outside this package and were not modified.

## Actionable findings

None. No correction is required before accepting this combined task delivery.

## Cannot independently verify / remaining limitations

- This is a read-only review of retained observations, not a new runtime execution. Snapshot sampling does not by itself prove continuous process exclusivity at every instant; same-service composition, the explicitly ordered stop/start report, stopped snapshot, and the separately closed Air gate support the scoped single-Go claim.
- The retained identity fingerprints prove unchanged record counts and identities across snapshots, not equality of every database field or media object byte. No broader data-content preservation claim is made here.
- The report states that temporary sources were restored byte-for-byte and typecheck exited zero; the scoped diff contains no proof UI edits and the restored screenshot supports the visible result. I did not rerun source baseline comparisons or typecheck, and the GREEN log alone does not encode an exit status.
- The final cache log directly records the edited title/socket rather than a second restored-title payload. Its restoration/readiness/installation claims are recorded in the final report and source-edit snapshots; these were not independently rerun.
- Only the local-override resolved Compose projection is retained; the report records also checking base+dev without the override. Inheritance is consistent with the actual files, but that second resolved output was not independently reproduced.
- Final-cache fresh cold-volume installation is not separately proven; the observed final restart uses populated cache volumes. The initial cold locked installation and final noninteractive cached restart are recorded separately.
- Task 7 still owns broad Go/manager checks, navigation/refresh and HMR reconnection after backend reload, Nuxt 503 recovery, final operational documentation, QA cleanup/original-data comparison, and fresh production-image proof from sanitized context. Tasks 5.1/5.2 remain unchecked at `tasks.md:24-25`. Existing compiled-image switch evidence does not prove a fresh production build of current code.

Full report path: `/home/obsidian/dev/wzap/.worktrees/enable-dev-live-reload/.superpowers/sdd/plan/task-4-6-review.md`.
