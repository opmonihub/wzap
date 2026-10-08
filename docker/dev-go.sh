#!/usr/bin/env bash
# Air remains the only watcher and builder. The inherited flock serializes Go
# executions even when Air's five-second wait expires before a drain finishes.
set -u
mkdir -p tmp
exec 9>>tmp/.dev-go.lock

if [[ ${1:-} == run ]]; then
    shift
    # Air signals this entire process group. A queued launch must cancel, while
    # an exec'd Go child receives that same signal directly, exactly once.
    trap 'exit 0' INT TERM
    flock -x 9 || exit $?
    trap - INT TERM
    # Air exits after five seconds and closes its PTY, which otherwise sends
    # SIGHUP and aborts a still-draining child. Preserve the original drain.
    trap '' HUP
    exec "$@"
fi

# Docker signals this parent; forward once to the exact Air child. Air alone
# signals Go. Keep the container alive until the inherited Go lock is released.
air_pid=
stopping=0
stop_air() {
    if [[ $stopping == 0 ]]; then
        stopping=1
        [[ -z $air_pid ]] || kill -TERM "$air_pid" 2>/dev/null || true
    fi
}
trap stop_air INT TERM
"$@" &
air_pid=$!
[[ $stopping == 0 ]] || kill -TERM "$air_pid" 2>/dev/null || true
status=0
while true; do
    wait "$air_pid"
    status=$?
    kill -0 "$air_pid" 2>/dev/null || break
done
flock -x 9
exit "$status"
