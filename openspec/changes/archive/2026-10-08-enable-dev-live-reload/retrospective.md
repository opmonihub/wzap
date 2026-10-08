# Retrospective — enable-dev-live-reload

Written once on2026-10-08 after implementation/runtime acceptance and controller QA cleanup, before independent Task7/whole-branch review and any PR. Later outcomes should be linked forward rather than rewriting this history.

## 0. Evidence

[verify.md](verify.md) records seven checks, with independent final reviews pending. [Task5 corrected review](evidence/task-5-round1-review.md) closes native lifecycle/pipe-output/duplicate-signal findings; [Nuxt/Compose review](evidence/task-4-6-review.md) approves the shared entrypoint and preserved switch. [Final gate matrix](evidence/task-7-gates.json), [Go integrations](evidence/task-7-go-integration.log), [117 manager tests](evidence/task-7-manager-test-final.log), [post-Go HMR](evidence/task-7-post-go-hmr.json), [503 recovery](evidence/task-7-503.json), [fresh production build](evidence/task-7-production-build-final.log), [production runtime](evidence/task-7-production-runtime.json), [healthy dev](evidence/task-7-return-dev.json), [original data/cleanup](evidence/final-cleanup.json) and [icon triage](evidence/task-7-icon-triage.md) bound the claims below.

## 1. Intent and delivered behavior

Go remains the public8081 entrypoint, API at the root and manager at /manager/. Optional development forwarding enables actual Vue/CSS HMR with automatic Go rebuild/restart; absence retains embedded production assets. The same Compose service/project reuses the existing infrastructure/data. No contract break, application dependency, migration, CI or application lifecycle rewrite was needed. Operational README covers both local-override forms and returning to compiled mode.

## 2. What worked

Contract tests separated configuration, HTTP and real WebSocket behavior before frontend integration. The mandatory native Air experiment revealed overlap before a wrapper was accepted; controlled eight-second drains and subsequent independent review validated the minimal guard and output/signal corrections. Shared Compose service replacement avoided a second application replica. Vault login, isolated test DB schemas, preserved runtime environment and sanitized git-archive production contexts allowed realistic acceptance without exposing credentials or changing original accounts. Final original counts/identities and infrastructure matched after QA removal.

## 3. What cost time or failed

Concurrent cold builds/tests produced existing manager5s timeouts; the unchanged full single-worker suite passed117/117. Heavy shared-host work should have been serialized earlier. A stale root-owned proof directory made the Go logger walker fail; preserving it outside the repo resolved the environment failure. The prescribed manager lint caught Nuxt key ordering; an initial move before modules needed correction to modules→development→ssr, with RED/FINAL GREEN retained.

Cold Nuxt/config-restart crossings caused temporary blank/login waits; recovered stable flows, not failed attempts, became acceptance. Vite automatic page reload discarded the page-local socket recorder, so its wait failed although reconnection had occurred; external sanitized lifecycle capture plus a subsequent automatic Vue edit supplied the right observation. The production helper ended143 with unknown cause after the switch, losing intermediate state; direct image/HTTP/browser capture verified the running production service. A return sampler omitted ps-e and initially counted0; the all-process sample proved oneGo. None of these harness failures was hidden or attributed to the application without evidence.

## 4. Decisions and trade-offs

Air stays the watcher/builder; protection was conditional on actual failure and does not introduce PID files, process-name kills or another build loop. Nuxt runtime hooks handle the installed schema/runtime Vite mismatch without dependencies; both hooks are development-only. Explicit503 avoids serving a stale bundle during Nuxt downtime. Dev cache volumes are separate from data volumes. Git-archive build contexts protect private scratch credentials without altering production packaging. Ordinary source edits reload automatically, while manager dependency changes explicitly recreate only Nuxt for locked installation.

## 5. Remaining review and limits

Independent Task7 and whole-branch review are next; no PR, merge, push or archive was performed. Production panel/routes/assets and navigation passed, but the sampled Instances screenshot has some unresolved glyphs: unchanged icon configuration includes asynchronous/external loading, and no icon-ready wait/network log was retained. Triage demonstrates no change-caused regression, not proven complete icon parity. Counts/hash comparisons do not prove every database field or media byte unchanged. The unknown helper143 cause remains unknown; missing intermediate artifacts are not claimed as retained evidence. Controller holds any final workflow/integration decisions.

## 6. Lessons for the next change

Serialize cold compilation, broad frontend tests and browser acceptance on the shared host. Make transient source proofs restoration-safe and record key state before later assertions. Use process sampling with all-process semantics. Keep reconnect evidence outside page memory when Vite may reload automatically; wait for the observable product state, with separate icon-readiness checks when claiming visual parity. Validate all prescribed gates early enough to catch configuration-only lint issues before the final production build. Preserve failed experiments and review outcomes alongside GREEN evidence so later reviewers can assess the actual guarantees.

## Forward note — review and completion

After this retrospective was written, Task7 review raised two README omissions (manager-dev environment reference and Air-config restart); both were corrected in ef170f8 and independently closed. The root dev setup line was committed separately while preserving user instruction edits. The separate GPT-6.1/max whole-branch [review](evidence/final-review.md) inspected all53packageparts/ten screenshots and approved specification, quality and readiness with no actionable findings. Verification now records seven PASS checks. Task-owned private scratch and remaining baseline auth vault were removed after that approval; dev remains the validated runtime. The bounded production icon/helper/preservation limits above remain unchanged. No integration, push, PR or archive has occurred.
