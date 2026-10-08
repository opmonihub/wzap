# Independent whole-branch review — enable-dev-live-reload

Review date: 2026-10-08. Worktree: `/home/obsidian/dev/wzap/.worktrees/enable-dev-live-reload`.

BASE: `1315b0e95aa3b45c1add393628f0b9095259dd0a`.
HEAD: `1cda2046ae826aab6af9a732e808906f537c1645`.

## Verdicts

- **Spec compliance: PASS.** The implementation and retained acceptance evidence support all eleven OpenSpec task checkboxes and the three delta specifications. The native Air failures justify the conditional guard. No unjustified scope deviation or unmet binding requirement was identified.
- **Whole-branch code quality: PASS.** No actionable Critical, Important or Minor finding was verified. The integrated proxy, Nuxt configuration, Air protection, Compose overlay and operational instructions are coherent; the previously reported findings are closed in the final code/documentation.
- **Ready for integration from this review's perspective: YES.** No correction is requested before integration. The evidence limitations below bound the claims, particularly production glyph readiness. The controller owns closing verification Check 2 and the subsequent integration/archive decision.

## Coverage and method

Read the controller's final brief, applicable root/manager instructions and Superpowers reviewer guidance. Read **every part 001–053 exactly once, in order, with complete bounded output and no truncation**. This covers the supplied package's commits, 100-file inventory, all code, planning/specification/documentation changes, metadata, historical and final raw proof/build logs, and binary-file entries. The manifest identifies a complete ordered partition of 468,804 bytes, SHA-256 `f3988eaf114fcf40f8370773d4436a3eb0359ea5c8da8f06d6b9467eecc51523`. Partition integrity is the supplied controller verification; no replacement diff, filtered substitute or history regeneration was used. **No package coverage gap remains.**

Opened and inspected all ten retained screenshots with `view_image`: the three Task 4/6 HMR/restoration captures and seven Task 7 desktop/mobile/HMR/unavailable/recovered/production captures. Supplementary reads were bounded to current neighboring router/middleware, static manager serving/tests, base Compose/package configuration, existing application shutdown and isolated Postgres fixture semantics. Current line references and HEAD were checked. `git status --short` contained only the preserved user-owned `AGENTS.md` modification and untracked `manager/AGENTS.md` before this report was written.

This review executed no application tests, browser sessions, Docker actions, DB/NATS queries or live-stack mutations. Test/build/runtime results below are **inspected retained evidence**, not newly executed checks. No source/index/branch mutation, further delegation, staging, commit, merge, push, PR or archive was performed. Only this review report was written. Dispatch requested `gpt-6.1-sol` with reasoning effort `max` and isolated context; requested settings are not independently verified runtime identity.

## Strengths and requirement assessment

### Configuration and public manager proxy — tasks 1.1–1.3

`internal/config/config.go:18`, `:99`, `:128` add the optional field, empty default and HTTP(S)-origin validation. Invalid URL errors at `:134` identify the variable without echoing its value or parser error. Tests cover absent/empty values, valid HTTP/HTTPS/root/IPv4/IPv6 origins, credentials, malformed/relative/opaque URLs, query/fragment markers and escaped path prefixes; reports retain observable RED/GREEN results.

`internal/httpapi/server.go:111` constructs the selected manager handler once. Unset configuration still uses the existing static handler; backend resource, auth, probe and Swagger registrations remain separate. `manager/devproxy.go:13` uses the standard reverse proxy's Rewrite path, changes only the origin, restores the original query, preserves public Host and reconstructs forwarding headers. `:23` supplies generic 503 transport failure without static fallback or sensitive error logging; `:34` preserves canonical redirect queries, including an empty marker.

The real HTTP tests at `internal/httpapi/manager_test.go:96`, `:147`, `:161`, `:206` cover escaped paths, queries, header sanitization, live/disk precedence, unavailable/recovered upstream and untouched backend routes. `manager/devproxy_test.go:14` adds actual TLS/body/status/hop-header behavior. `internal/httpapi/manager_websocket_test.go:22` traverses the composed router and middleware with a real TCP upgrade, checks 101/accept/subprotocol/public Host/path/query, then exchanges an unsolicited upstream frame and masked client/reply frames. The static-branch RED and focused/race GREEN records substantiate the regression. Existing middleware skips manager access logging and exposes ResponseController unwrapping; no request token/query logging regression was found.

### Public Nuxt HMR — task 2.1

`manager/nuxt.config.ts:13` scopes both hooks to development. The font hook aligns emitted URLs with the already-prefixed handler; the client-only Vite hook at `:21` applies the public client port without adding another manager prefix. Existing SSR false, `/manager/`, empty same-origin API base and standalone Vite API proxy remain intact. The installed-version schema/runtime workaround is documented with its precise typecheck RED and final GREEN; no dependency change was introduced.

`evidence/task-4-6-upgrade.json` records a real public 101 on `ws://127.0.0.1:8081/manager/_nuxt/`. Task 4/6 HMR metadata records received traffic, automatic Vue/CSS changes and successful prefixed font requests. Desktop/mobile proof screenshots visibly show the temporary magenta title, and the restoration capture shows Overview with normal color. Final cache-isolated HMR evidence repeats an automatic edit after the configuration's final cache mount was applied.

Task 7's external `task-7-post-go-hmr.json` records lost → connecting → connected and the next Vue edit without navigation at the public origin; its screenshot shows the updated title/color. This supplies the relevant acceptance observation after the page-local recorder was lost during Vite's automatic reload.

### Air reload, serialization and shutdown — tasks 3.1–3.3

`.air.toml:7`, `:8`, `:13` retain Air as the direct watcher/builder, include Go/mod/sum/SQL, exclude tests/frontend caches and allow a 12-second interrupt grace. `Dockerfile.dev:7` pins Air v1.61.7 and checks required shell/lock tools. The application still has its existing 10-second shutdown budget and component order (`cmd/wzap/main.go:48`, `:474`); Compose's dev grace is 20 seconds.

Native raw records demonstrate replacement starting before an eight-second child exited and supervisor exit around five seconds with active children; native watch/build-error recovery also passed. The conditional protection therefore meets the design's prerequisite. `docker/dev-go.sh:13` serializes exec'd Go children using inherited kernel locking, cancels queued launches on shutdown, ignores the demonstrated PTY HUP, and at `:46` keeps the parent alive through lock release. Air remains the only watcher/builder; there are no PID files, process-name kills or application lifecycle changes.

The prior I1 and M1 are closed in the actual integrated implementation. `docker/dev-go.sh:37` applies O_APPEND to Air's real, separate inherited output destinations. Protected fixtures use ordinary separate pipes, so collector flags cannot hide the old runtime stall; the adverse single-CPU RED and three GREEN reload/error/recovery runs are retained. `:26` and `:39` share idempotent stop forwarding; `TestDevSupervisorSignalOnce` at `docker/dev_reload_test.go:122` observes every synthetic Air signal instead of relying on Air's own one-shot handler.

The six-case final host/container logs and exact prior reviewer reproducer pass. Actual disposable Docker stop/reload records retain one signal, at least eight-second drain, previous exit before latest start, child exit before container exit and stop durations below 20 seconds. The ordinary final Go suite correctly distinguishes its opt-in Air skip from these executed dedicated proofs.

### Shared Compose workflow and preservation — tasks 4.1–4.2

`docker-compose.dev.yml:4` overlays the existing `wzap` service with a distinct dev image, inherited public port/data volume/infrastructure and a dev healthcheck. `:20` supplies the required grace; `:22` starts Node 24 with frozen installation and the manifest-pinned pnpm, without a profile or host port. Dependency, store and Nuxt caches are separate from data volumes. Both resolved file sets in `task-7-compose.json` confirm the same project, one Go service, 8081 and internal-only frontend.

Complete runtime records show unchanged infrastructure IDs/ports/volume names and unchanged application/manager container IDs across source edits. The Task 4/6 stopped snapshot precedes compiled startup. Task 7's fresh production/returned-dev snapshots preserve those infrastructure and identity/count fingerprints; final cleanup returns the original one user/one instance after removing only disposable QA. The final all-process sample records one Go serve, healthy dev, readyz ready and manager 200. No committed domain/data-writing change, new dependency, migration, CI edit or production Dockerfile change appears in the branch.

### Final gates, production and documentation — tasks 5.1–5.2

Inspected the full final gate matrix and raw outputs: tracked Go formatting, vet, golangci-lint v2.13.2 and build record exit 0. The final full Go suite records both test URLs explicitly set, reachable `wzap_test`/NATS and all packages passing. The existing Postgres helper validates the `_test` suffix, creates a unique schema/search_path and drops it; final retained schema count is zero. The initial unreadable task-scratch failure is preserved separately from the successful run.

The final manager suite records 117/117 tests in 14 files with one worker. Initial contention-sensitive parallel timeouts remain visible. Final lint records zero errors/two existing prop warnings after the development block's ordering correction; typecheck and static build record exit 0. No Swagger annotations/generated documents changed, so no generation claim is made. Strict OpenSpec validation records success. Source/docs/metadata whitespace checks explicitly exclude byte-preserved raw logs with terminal trailing whitespace.

Browser reports and captures substantiate actual desktop/mobile login, direct internal navigation/refresh, automatic updates, public reconnection and Nuxt-only 503/recovery with backend health intact and the same Go container. All ten screenshots were inspected in this review.

The final production log builds from committed runtime code `3b950c1` through the unchanged production stages and names the digest matching the actual production snapshot. Later commits contain documentation/evidence/instructions only. With Nuxt exited, production manager root/internal route and nonempty CSS/three JS assets return 200 with appropriate types, and the compiled Instances screenshot renders its layout/content. Sanitized archive contexts exclude private scratch. The final return record distinguishes the missing intermediate sampler output from the corrected all-process capture.

`README.md:67`, `:560`, `:597`, `:605`, `:622` document the optional origin, both Compose file sets, logs/startup, locked manager reinstall, explicit Air-config restart, expected brief outage and ordered stop of both dev services before compiled startup. The two Task 7 P3 documentation findings are closed. The package's root AGENTS change is exactly one isolated authorized setup line; the user's larger root rewrite and untracked manager instructions remain outside the commits. `tasks.md` is fully checked; `verify.md` deliberately keeps final review Check 2 pending and the retrospective preserves historical limits.

## Actionable findings

### Critical

None verified.

### Important

None verified.

### Minor

None verified. Previously closed task findings are not deferred or reopened by assumption.

## Evidence limitations and recommendations

- **Production glyph readiness/parity remains unverified.** The production screenshot visibly has blank view-toggle/empty-state/plus glyphs. Its heading-ready capture has no retained icon-ready wait or production network outcome, and the older compiled baseline is another view. Unchanged manager sources/dependencies, development-only new hooks and documented asynchronous Iconify behavior do not prove the exact missing-glyph cause or same-view baseline. Route/layout/embedded-assets acceptance is supported; complete production visual parity is not claimed. A separate icon-ready/network capture is appropriate before making that broader claim, but no branch-caused code defect or required correction is demonstrated here.
- **The helper exit 143 cause is unknown.** Missing intermediate production/return snapshots are not audited as retained proof. Direct final image/HTTP/browser/process records support current behavior; they do not recreate the absent chronological samples. Same-service composition, explicit ordered stop assertions, the retained Task 4/6 stopped state and Air's controlled lifecycle proof support the required exclusivity without pretending sampled metadata is a continuous trace.
- **Preservation is bounded to recorded counts/identity hashes and infrastructure/mount/port comparisons.** It does not prove byte equality of every database field or media object. No broader equality claim is used for approval.
- **This is a read-only evidence review.** Local runtime, integration URLs, build/test exits and source restoration were not rerun here. Full retained outputs were inspected rather than trusting status summaries alone; fresh runtime claims belong to the recorded executions.

## Declined to judge

None. No considered behavior was silently set aside as outside the specification. Unverifiable observations are explicitly bounded above rather than discarded or assigned an unsupported severity.

## Assessment

The final branch satisfies the planned development behavior while keeping production composition and the existing application lifecycle intact. Real HTTP/WebSocket tests, native-versus-protected Air proof, actual public HMR/reconnect/recovery, explicit isolated integrations, a fresh production image and recorded preservation support approval. **No open actionable finding; no required implementation change.**
