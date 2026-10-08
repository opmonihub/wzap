### Spec compliance

- ❌ Issues found: task 3.1's automatic Go build/restart is unreliable with the protected launcher and ordinary stdout/stderr pipes. A focused synthetic check against the actual script stalled on its first Go edit: no termination signal or replacement within 12s. The harness's O_APPEND output at `docker/dev_reload_test.go:158` avoids the failing Air path, while the runtime command at `Dockerfile.dev:15` leaves it reachable. Evidence: `.superpowers/sdd/plan/task-5-review-stdio.log:2` and `:3`.
- ✅ The requested configuration is present: direct Go build, Go/mod/sum/SQL watches, docs Go observed, tests/frontend/cache exclusions, 200ms debounce, interrupt delivery and 12s kill delay (`.air.toml:5`, `:8`, `:9`, `:10`, `:11`, `:13`, `:15`).
- ✅ Native RED precedes the conditional protection; the authoritative native log fails four lifecycle scenarios, with native watch/recovery passing (`.superpowers/sdd/plan/task-5-red-final-host.log:139`, `:142`). The protection uses inherited kernel locking and an exact Air child, with no own watch/build loop, PID files, process-name kills, application dependencies, migrations, CI edits or application lifecycle changes (`docker/dev-go.sh:6`, `:13`, `:18`, `:28`, `:37`, `:41`; review diff file inventory).
- ✅ Recorded protected host and dev-container harness runs pass all five requested scenarios. Three actual Docker proofs on standard stdout pass normal stop, stop during drain, and successive saves through the latest replacement and subsequent stop (`.superpowers/sdd/plan/task-5-green-final-host.log:51`, `task-5-green-final-docker.log:51`, `task-5-runtime-baseline.log:1`, `task-5-runtime-baseline.log:2`, `task-5-runtime-reload.log:1`). These are valid positive lifecycle evidence; they do not rule out the independently reproduced watch stall.
- ⚠️ The unchanged application 10s shutdown and eventual Compose 20s grace cannot be verified from this task diff. The Docker evidence explicitly uses `--stop-timeout 20`/`docker stop --time 20` (`.superpowers/sdd/plan/task-5-runtime-baseline.py:29`, `:37`); final Compose wiring belongs to task 4. No change to application shutdown appears in this diff.

### Code-quality verdict

- **Needs fixes.** The lock/drain implementation addresses the demonstrated lifecycle defects, but capture-only suppression of the upstream logging defect leaves a real automatic-reload failure in the runtime path.

### Strengths

- The execution lock survives Go exec and temporary-directory cleanup, making replacement depend on release by the previous process rather than its pre-exit log record (`docker/dev-go.sh:6`, `:13`, `:18`, `:41`).
- HUP handling is limited to the demonstrated PTY hangup, while INT/TERM still reach the exact Go execution through Air (`docker/dev-go.sh:12`, `:14`, `:17`, `:18`).
- The harness uses real pinned Air and isolated synthetic programs, checks ignored edits by build count, verifies corrected builds/latest content, and checks signal count, drain duration, overlap and final PID disappearance (`docker/dev_reload_test.go:20`, `:29`, `:37`, `:49`, `:85`, `:93`, `:116`, `:272`).
- The report distinguishes expected native failures, final GREEN evidence, skipped ordinary runs and the known upstream limitation instead of claiming nonexistent Postgres/NATS checks (`.superpowers/sdd/plan/task-5-report.md:37`, `:41`, `:68`, `:81`).

### Issues

#### Critical

- None found.

#### Important

- **I1 — Runtime automatic reload can stall; the new capture mode conceals the failure.** `Dockerfile.dev:15`, `.air.toml:7`, `docker/dev_reload_test.go:158`. Air v1.61.7 returns the same PTY as both stdout and stderr (`/home/obsidian/go/pkg/mod/github.com/air-verse/air@v1.61.7/runner/util_linux.go:31`), then starts two readers (`runner/engine.go:613`, `:618`). Go1.26.0's zero-copy attempt acquires the destination stdout write lock before the PTY read lock (`/home/obsidian/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.linux-amd64/src/internal/poll/copy_file_range_unix.go:55`, `:59`). If the other reader is waiting for output from a quiet child, Air's change-event log blocks before `stopBin` and `buildRun` (`runner/engine.go:360`, `:373`, `:375`). The captured earlier stack shows precisely that chain (`.superpowers/sdd/plan/task-5-stall-dump.log:69`, `:88`, `:357`). O_APPEND bypasses zero-copy (`.../src/os/zero_copy_linux.go:48`) only in the test capture; the runtime wrapper does not contain this defect. The recorded native Docker watch timeout (`task-5-docker-stop-final.log:7`) corroborates the runtime concern but has no stack trace proving its cause. Crucially, the reviewer directly reproduced the functional failure through the current protected script with normal pipes: initial child started, first Go save produced neither signal nor replacement within 12s, stdout stopped at `running...` (`task-5-review-stdio.log:2`, `:3`, `:4`). The Docker GREEN runs remain valid, but three successful schedules cannot invalidate a reachable failure. Resolve the runtime I/O path with minimal dev-only protection, keep pinned Air/log visibility/no custom watcher or build loop, and test that same path before marking 3.1 complete.

#### Minor

- **M1 — A narrow startup window can forward TERM to Air twice.** `docker/dev-go.sh:28`, `:34`. If a signal arrives after assigning `air_pid` and before the following stopping check, `stop_air` forwards it and the fallback statement forwards it again. The steady-state flag prevents later forwards, and Air serializes its Go-child stop requests, so no duplicate Go signal is demonstrated. Make the startup fallback and trap share one idempotent forwarding function/flag; current lifecycle tests stop only after the synthetic child has started.

### Checks and evidence

- Read the requested diff exactly once: `.superpowers/sdd/plan/review-31434f2..7624124.diff` (base `31434f2`, head `7624124`). Read the brief/report and retained authoritative logs; did not rerun their suites, issue git commands, mutate code/index/branch, dispatch agents or act on a live stack.
- Named outside-diff risk: **O_APPEND hides a reachable runtime PTY/stdout lock blockage.** Checked only pinned Air's event/PTY/copy/stop implementation, the local Go zero-copy lock path, and the specifically cited diagnostic/runtime evidence. The normal-stdout proof scripts genuinely attach ordinary Docker stdout (`task-5-runtime-baseline.py:29`, `task-5-runtime-reload.py:29`).
- Ran one focused temporary synthetic fixture using actual `docker/dev-go.sh`, Air v1.61.7/Go1.26.0, separate stdout/stderr pipes, a quiet child, one Go edit and a 50ms drain. Result: **RED**, exit 1, no automatic replacement within 12s; supervisor cleanup exited 0. This is a new stdio-path check, not a repetition of the O_APPEND suite. Original output is retained as `task-5-review-stdio.log`; no real wzap/DB/NATS/active tmp path was used.
- Minimal reproducer: `python3 .superpowers/sdd/plan/task-5-review-stdio.py` from this worktree. It runs only the first Go-edit case, is scheduling dependent like the upstream race, uses independent `/tmp` fixtures, identifies exact synthetic PIDs for fallback cleanup, and writes subsequent output to `task-5-review-stdio-last.log` so the original RED evidence is preserved. The saved minimized form passed Python syntax/root-resolution checks and was not rerun during review.

### Assessment

- **Spec compliance: Issues found. Code quality: Needs fixes.** The required configuration, native-first decision and protected shutdown/serialization proofs are sound. The reproduced automatic-watch stall blocks closure of task 3.1 and task trust until the runtime path, rather than only its test capture, is protected.
