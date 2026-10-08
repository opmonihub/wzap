# Task 5 — Air reload and shutdown (OpenSpec 3.1–3.3)

Status: complete for implementation; independent review remains the controller’s next gate.

## Scope and safety

Only `.air.toml`, `Dockerfile.dev`, `docker/dev-go.sh`, `docker/dev_reload_test.go` and the three authorized OpenSpec task checkboxes are eligible for the code commit. Existing root and manager AGENTS edits are unrelated and remain unstaged. Every execution uses a synthetic Go main in its own temporary directory; it never runs real wzap, accesses a DB, uses the active service tmp directory, or switches the current stack. Disposable Docker proof containers have no DB/network configuration. The dev image was rebuilt without restarting any stack container.

## Native RED and root causes

The native test uses pinned Air v1.61.7, an `exec` Go child, 200ms debounce, interrupt delivery and 12s kill delay. Initial proof precedes protection (`task-5-native.log`, reruns `task-5-native-final.log`, `task-5-red-final-host.log`). In the native drain proof the replacement began +6.992s while the original did not record exit until +8.223s. Native successive edits likewise produced the latest version while the old child was draining. Native supervisor exit occurs around five seconds, with active children. Compilation-error recovery and scoped watch triggers passed natively.

Source inspection of Air v1.61.7 (`runner/engine.go`, stopBin lines 610–633 in the design's pinned source) confirms its fixed five-second wait. `runner/util_linux.go` sends one process-group interrupt, sleeps the configured kill delay, then SIGKILLs the group. The five-second supervisor wait therefore is independent of the configured 12s grace.

A first lock-only protection serialized replacement launches but did not preserve child shutdown: Air exits and closes its PTY, generating SIGHUP at five seconds. `task-5-hup-proof.log` captured a second signal at +5.159s after the original signal +0.105s, then the deliberately HUP-aware child exited +8.110s. Without HUP interception a child aborts early and releases its lock. Production wzap handles INT/TERM (`cmd/wzap/main.go:95` and `:628`); the protection ignores inherited HUP before exec while leaving INT/TERM unchanged. No second INT/TERM is forwarded to Go.

Docker native normal stop also records only start+signal and container exit before the eight-second child exit (`task-5-docker-stop.log`: 5.692s, RED). A native stop during drain records the same RED result (5.190s). These are real `docker stop --time 20` calls against disposable containers.

## Conditional protection

`docker/dev-go.sh` uses one kernel flock inherited by the exact exec'd Go process. Replacement launches wait on that lock and canceled queued launches exit on Air's process-group signal. The parent forwards a single termination signal to its exact Air child, waits for Air, then waits on the same lock before exiting. No PID files, process-name lookup, custom watcher, custom build loop, or duplicate Go signal are used.

The lock descriptor is opened before Air starts, so removing its path during cleanup does not lose the held inode. `clean_on_exit=true` and `stop_on_error=true` remain the existing native defaults: no cleanup/error override was retained without evidence. The harness tests correction after invalid compilation and queued launches during drain.

Watch extensions are go/mod/sum/sql. Frontend extensions and tests are excluded; dependency/output/cache directories are excluded, but Go files in manager and generated Go in docs remain observed. The test checks build count, rather than merely restart count, for ignored edits.

## Diagnostic correction to the test logger

Some first runs appeared to miss watch events. The captured Air goroutine dump (`task-5-stall-dump.log`) showed the event had already arrived: main blocked while logging it at engine.go:360. Air starts two io.Copy routines reading the same PTY, and Go 1.26's zero-copy optimization could lock stdout while waiting for the PTY read lock held by the other routine reading the silent child. The initial harness used os.Create for air.log. Opening the capture log with O_APPEND skips that zero-copy path while preserving every build and error message. The read-only diagnostic agent supplied the lock-chain analysis from this dump; it was interrupted after its findings when the user updated model routing, so no separate diagnostic report file is claimed. This was investigated rather than changing timeouts, skipping failed events, adding file polling, or manually touching source to recover a test.

Air -d was briefly used for evidence capture and removed: logging every air.log write generates self-feedback in a watched directory. Temporary stack/strace instrumentation was also removed from the deliverable harness.

## Commands and outputs

Local toolchain: `/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin/go` (go1.26.0). Local Air: `/home/obsidian/go/bin/air` (v1.61.7). Container image provides Go1.26.8 and Air v1.61.7.

```
PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air WZAP_TEST_AIR_NATIVE=1 go test ./docker -run TestDevReload -count=1 -v
# task-5-red-final-host.log — expected native RED, FAIL74.131s: four lifecycle failures, watch/recovery PASS39.58s

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air go test ./docker -run TestDevReload -count=1 -v
# task-5-green-final-host.log — PASS90.854s, all five scenarios

docker run --rm --entrypoint sh -v "$PWD:/src" -w /src -e TMPDIR=/src/.superpowers/sdd/plan/reload-temp -e WZAP_TEST_AIR_BIN=/go/bin/air wzap:dev-live-reload -c 'go test ./docker -run TestDevReload -count=1 -v'
# task-5-green-final-docker.log — PASS91.839s, all five scenarios; Go1.26.8/Airv1.61.7

python3 .superpowers/sdd/plan/task-5-docker-proof.py
# task-5-docker-stop.log — native normal stop RED5.692s; native during-drain RED5.190s; first protected normal stop GREEN8.381s. Initial cleanup hit root-owned fixture paths; the final proof scripts chown only their disposable fixture before removal.

python3 .superpowers/sdd/plan/task-5-runtime-baseline.py
# task-5-runtime-baseline.log — actual Docker standard stdout; protected normal stop GREEN8.888s; during-drain stop GREEN8.106s

python3 .superpowers/sdd/plan/task-5-runtime-reload.py
# task-5-runtime-reload.log — actual Docker standard stdout; successive edits -> latest replacement -> stop GREEN8.243s; old exit+8.332s precedes latest start+8.335s; latest exit+16.408s precedes container exit

docker build -f Dockerfile.dev -t wzap:dev-live-reload .
# task-5-image-build.log — exit 0, image exported and unpacked

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH go test ./docker -run TestDevReload -count=1 -v
# PASS; dedicated Air test SKIP without WZAP_TEST_AIR_BIN, as required

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH go vet ./docker
bash -n docker/dev-go.sh
git diff --check
# exit 0
```

Full logs are retained under this report directory. No Postgres or NATS integration test is claimed.

## Delivery evidence

- `.air.toml:5`: direct Go build; `:7` conditional protection launch; `:8` observed extensions; `:9` excluded frontend dependencies/output/caches; `:11` 200ms debounce; `:13` 12s interrupt grace; `:18` original cleanup retained.
- `Dockerfile.dev:7`: pinned Air/tool availability check; `:15` dev-only protected parent.
- `docker/dev-go.sh:6`: descriptor-based lock; `:12`: cancel queued launches; `:17`: preserve drain across Air PTY hangup; `:28`: one signal to exact Air PID; `:37`: wait for Air; `:41`: stay alive until exact Go execution releases its inherited lock.
- `docker/dev_reload_test.go:20`: real pinned Air requirement/opt-in; `:29`: 8s drain; `:37`: successive saves/latest content; `:49`: ignored-build scope and recovery; `:85`: stop during drain; `:93`: supervisor stop; `:116`: temp synthetic fixture; `:158`: append-mode capture; `:272`: timeline/signal/overlap/drain/reaping assertions.

All lifecycle assertions log start/signal/exit chronology. Every protected child completed at least its configured eight-second drain, received one INT/TERM, and was reaped before cleanup. The no-Air invocation explicitly SKIPs the dedicated proof while ordinary tests pass.

## Upstream limitation at 7624124 (resolved by fix round 1)

O_APPEND is a fixture capture correction. It does not change Air’s two-copy shared-PTY implementation or establish that every Docker stdout workload is immune to the Go zero-copy lock inversion. The actual unredirected Docker runtime was separately tested for normal stop, stop during drain, and successive edits through the latest replacement and subsequent stop, all GREEN. An earlier native Docker runtime watch request did stall (`task-5-docker-stop-final.log`); no global absence of this upstream logging race is claimed. At commit 7624124, production output redirection, an Air upgrade, and polling were not added. Independent review reproduced the runtime stall and required the fix recorded below before live activation.

## Evidence inventory

Final authoritative logs: `task-5-red-final-host.log`, `task-5-green-final-host.log`, `task-5-green-final-docker.log`, `task-5-runtime-baseline.log`, `task-5-runtime-reload.log`, `task-5-image-build.log`, `task-5-final-static.log`. Reproducible Docker scripts: `task-5-docker-proof.py`, `task-5-runtime-baseline.py`, `task-5-runtime-reload.py`.

Supporting RED/debug logs: prior `task-5-native.log`; reruns `task-5-native-current.log`, `task-5-native-bind.log`, `task-5-native-debug.log`, `task-5-native-host.log`, `task-5-native-final.log`; protected attempts `task-5-protected-host.log`, `task-5-green-host.log`, `task-5-green-clean.log`, `task-5-green-docker.log`; stop/HUP investigation `task-5-stop-debug.log`, `task-5-lock-debug.log`, `task-5-lock-debug2.log`, `task-5-shell-trace.log`, `task-5-strace.log`, `task-5-strace-test.log`, `task-5-strace-test2.log`, `task-5-strace-test3.log`, `task-5-hup-proof.log`; watcher diagnostics `task-5-watch-debug.log`, `task-5-stall-dump.log`, `task-5-stall-abrt.log`; Docker preliminary attempts `task-5-docker-stop.log`, `task-5-docker-stop-final.log`. All paths are relative to `/home/obsidian/dev/wzap/.worktrees/enable-dev-live-reload/.superpowers/sdd/plan/`.

Commit: `76241247157e16211732a75a318832a132836a5b` (`fix(dev): serialize Air reloads and preserve graceful shutdown`). The commit contains exactly the five authorized code/test/task files; git status retains only unrelated root AGENTS edits and untracked manager AGENTS. No task outside 3.1–3.3 was marked by this implementer.


# Task 5 fix round 1 — runtime pipe output

Status: fixed and covering verification restored; independent re-review is the next controller gate. Base: `76241247157e16211732a75a318832a132836a5b`.

## I1: verified runtime defect, RED, and minimal fix

The independent review (`task-5-review.md`, `task-5-review-stdio.log`) reproduced the stdout lock inversion through the real protected runner with separate ordinary stdout/stderr pipes. The original fixture's append capture hid that production path. Task checkboxes 3.1–3.3 were reopened during this round and restored only after the covering checks passed.

The new regression runs a quiet artificial child with separate pipe descriptors and `GOMAXPROCS=1`; that adverse reader scheduling reproduced the first Go edit stalling on the old script. `task-5-round1-red-singlecpu.log`: FAIL27.964s, timeout waiting for `start pipe-reload`, with only initial `start one` and no termination signal. An initial scheduling-dependent unconstrained attempt passed (`task-5-round1-red-pipes.log`); it is not claimed as RED. The one-CPU setting makes the hazardous ordering repeatable without runtime instrumentation, a custom watcher, or manually retriggering edits.

`docker/dev-go.sh:37` now launches Air with `>>/proc/self/fd/1 2>>/proc/self/fd/2`. Reopening the inherited destinations with O_APPEND makes Go's `os.File.readFrom` skip the zero-copy path (`os/zero_copy_linux.go:42–50`, `os/file_unix.go:93–98`) that locks stdout before the shared PTY reader. Stdout and stderr keep their separate destinations, including Docker pipes. No log tailer/relay, dependency, Air version change, build/watch loop, PID protocol, process-name lookup, or stack change was added. The existing flock, INT/TERM delivery, ignored PTY HUP and eight-second drains are preserved.

All protected Go fixtures now pass ordinary separate pipes to the actual runner. Only the collector file is append-mode; those flags do not propagate through the pipes. Native lifecycle characterization deliberately keeps append capture to isolate the independently established native five-second/overlap defect, and the new runtime-protection-specific regression is skipped in native mode. The regression asserts automatic reload, visible compilation failure, automatic recovery, exact signal count, drain, ordering and child cleanup. It cannot pass by applying append flags only to the capture file.

## M1: shared idempotent forwarding

The startup fallback and signal trap now both call `stop_air`; `signal_sent` is set before forwarding to the exact Air PID (`docker/dev-go.sh:25–30`, `:39`). A signal before PID assignment records the stop request, and the shared fallback forwards once after assignment. A signal after assignment forwards through the same guarded helper, so the fallback cannot duplicate it.

`TestDevSupervisorSignalOnce` (`docker/dev_reload_test.go:122`) uses a controlled synthetic Air child whose signal handler records every delivered TERM. It sends a second parent TERM while that child drains and verifies one forwarded signal and child exit before the parent. This focused check bypasses real Air's own one-shot signal handler so it cannot conceal duplicates. It checks repeated-parent-signal forwarding, without claiming a forced inter-statement reproduction of the prior startup window. Count3 PASS5.386s; final cleanup revision count1 PASS0.878s (`task-5-round1-green-signal-once.log`, `task-5-round1-green-signal-final.log`).

## Covering commands and results

All paths below are relative to `.superpowers/sdd/plan/` in this worktree. No real wzap, DB, NATS, active tmp directory, application stack, or infrastructure container was launched or mutated.

```
PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air go test ./docker -run 'TestDevReload/PipeOutputReloadAndRecovery$' -count=1 -v
# Before runtime fix: task-5-round1-red-singlecpu.log — expected RED27.964s

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air go test ./docker -run 'TestDevReload/PipeOutputReloadAndRecovery$' -count=3 -v
# After runtime fix: task-5-round1-green-pipes.log — PASS53.212s, three consecutive regression runs

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air go test ./docker -run TestDevReload -count=1 -v
# task-5-round1-green-host.log — PASS109.680s, all six cases with real pipes for protected fixtures

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH go test ./docker -run TestDevSupervisorSignalOnce -count=3 -v
# task-5-round1-green-signal-once.log — PASS5.386s
# Final cleanup-only revision rerun count1: task-5-round1-green-signal-final.log — PASS0.878s

docker run --rm --entrypoint sh -v "$PWD:/src" -w /src -e TMPDIR=/src/.superpowers/sdd/plan/reload-temp -e WZAP_TEST_AIR_BIN=/go/bin/air wzap:dev-live-reload -c 'go test ./docker -run "TestDev(Reload|SupervisorSignalOnce)" -count=1 -v'
# task-5-round1-green-docker.log — PASS112.141s; Go1.26.8/Airv1.61.7; all six reload cases and synthetic-Air signal forwarding

PATH=/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/bin:$PATH WZAP_TEST_AIR_BIN=/home/obsidian/go/bin/air WZAP_TEST_AIR_NATIVE=1 go test ./docker -run 'TestDevReload/(Drain|SupervisorStop)$' -count=1 -v
# task-5-round1-native.log — expected native RED25.581s: Drain, StopDuringDrain and SupervisorStop. The selector also matches the Drain suffix in StopDuringDrain. Native implementation is unchanged; full prior native watch/other lifecycle evidence remains task-5-red-final-host.log.

python3 .superpowers/sdd/plan/task-5-round1-runtime-baseline.py
python3 .superpowers/sdd/plan/task-5-round1-runtime-reload.py
# Actual Docker standard stdout, quiet synthetic child, GOMAXPROCS=1, --stop-timeout20 and docker stop --time20:
# task-5-round1-runtime-baseline.log — GREEN normal stop8.429s; during-drain stop8.131s; one signal and >=8s drain
# task-5-round1-runtime-reload.log — GREEN successive saves -> latest replacement -> stop8.217s; old exit+8.378s < latest start+8.383s; latest exit+16.484s precedes container exit

GOMAXPROCS=1 python3 .superpowers/sdd/plan/task-5-review-stdio.py
# task-5-round1-review-repro-green.log and task-5-review-stdio-last.log — GREEN; one Go edit automatically signals/exits initial child and starts version two; supervisor cleanup exit0
```

The dev image/Dockerfile did not change in this round. The real runtime script is bind-mounted into disposable proof containers, matching the supported dev workflow. No image rebuild or unrelated full suite was needed. All temporary runtime proof containers were removed. Root/manager AGENTS and root-owned untracked OpenSpec evidence remain unstaged.

## Final evidence and remaining boundaries

- Runtime I/O guard: `docker/dev-go.sh:34–37`; single signal forwarding: `:25–30`, `:39`; original exec lock/HUP drain guard: `:6`, `:13`, `:17–18`, `:46`.
- Pipe regression/adverse scheduling: `docker/dev_reload_test.go:30–46`; synthetic exact-Air signal count: `:122`; plain inherited pipes and collector-only capture flags: `:223–258`; lifecycle checks: `:364`.
- Prior capture-only limitation is now addressed in the runtime launcher. Air's upstream shared-PTY implementation still exists, but the dev runtime deliberately disables the particular Go zero-copy lock path rather than claiming Docker is immune by chance.
- Compose 20s grace and live-stack/HMR verification still belong to later tasks; this round proves disposable Docker stop with an explicit 20s timeout. Independent review remains required before controller progression.

Round-1 commit: `04e1af98837a2891e00f5f41fa853c61448b709f` (`fix(dev): prevent Air pipe-output reload stalls`). Exactly two files were committed: docker/dev-go.sh and docker/dev_reload_test.go. Checkboxes 3.1–3.3 were restored after verification and have no net diff against the base commit. Final go vet ./docker, bash -n, gofmt -l (empty), and scoped git diff --check each returned exit0. task-5-round1-static.log records dedicated Air SKIP without WZAP_TEST_AIR_BIN and synthetic-Air forwarding PASS1.318s. Unrelated AGENTS and OpenSpec evidence remain unstaged. No pending test runs or disposable dev-image containers remain.
