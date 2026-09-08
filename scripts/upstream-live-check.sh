#!/usr/bin/env bash
# Run the live suite against a real Super Productivity.
#
# Takes a checkout of Super Productivity that has already been built
# (buildFrontend + electron:build) and starts it headless with its Local REST
# API forced on, then runs `go test -tags live` against it.
#
# This exists because the bridge's own CI cannot see Super Productivity change.
# SP began requiring an access token in 18.19.0 and every published release of
# the bridge was broken against it for a month (#64); CI was green throughout,
# correctly, because it tests against fixtures and a stub. Only a real SP
# catches SP changing.
#
# Exit codes are load-bearing — the workflow decides whether to file an issue
# from them:
#   0  the suite passed
#   1  the suite failed: a real finding about the bridge
#   2  the environment never came up: says nothing about the bridge
set -uo pipefail

usage() {
    echo "Usage: upstream-live-check.sh <built-sp-checkout>"
    echo
    echo "  Starts that Super Productivity headless with its Local REST API"
    echo "  forced on, seeds a throwaway profile, and runs the live suite."
    echo
    echo "Exit: 0 suite passed, 1 suite failed, 2 environment did not come up."
}

case "${1:-}" in
    -h | --help) usage; exit 0 ;;
esac

if [ "$#" -ne 1 ]; then
    usage >&2
    exit 2
fi

SP_DIR="$1"
REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

# Not a secret. The API binds loopback only, the profile is thrown away at the
# end of the job, and SP generates a random one when this is unset — which
# would then have to be scraped out of its log.
TOKEN="${SP_FORCE_LOCAL_REST_API_TOKEN:-CiTokenForUpstreamLiveCheck1234x}"
BASE="http://127.0.0.1:3876"
PROFILE="${SP_CI_PROFILE:-$SP_DIR/.ci-profile}"
LOG="${SP_CI_LOG:-$SP_DIR/.ci-sp.log}"
READY_TIMEOUT="${SP_CI_READY_TIMEOUT:-120}"

SP_PID=""
cleanup() {
    if [ -n "$SP_PID" ]; then
        kill "$SP_PID" 2>/dev/null
        wait "$SP_PID" 2>/dev/null
    fi
}
trap cleanup EXIT

fail_env() { echo "upstream-live-check: $*" >&2; exit 2; }

[ -d "$SP_DIR" ] || fail_env "no such directory: $SP_DIR"
FRONTEND="$SP_DIR/.tmp/angular-dist/browser/index.html"
[ -f "$FRONTEND" ] || fail_env "no built frontend at $FRONTEND (run buildFrontend first)"
[ -f "$SP_DIR/electron/main.js" ] || fail_env "no built electron main (run electron:build first)"

# The port is hard-coded in SP (LOCAL_REST_API_PORT) with no override, so a
# second Super Productivity anywhere on the machine silently answers these
# requests instead. Without this check a developer's own running SP makes the
# whole run a false pass — which is exactly what happened the first time this
# was tried by hand.
if curl -s --max-time 2 "$BASE/health" >/dev/null 2>&1; then
    fail_env "something already answers on $BASE — refusing to test against another Super Productivity"
fi
echo "==> nothing on $BASE; the instance under test will be ours"

rm -rf "$PROFILE"
mkdir -p "$PROFILE"

# NODE_ENV=DEV is what gates SP's force flag (isForceEnabledForDev). --custom-url
# overrides the DEV branch that would otherwise load http://localhost:4200, so a
# built frontend is enough and no `ng serve` is needed alongside.
(
    cd "$SP_DIR" || exit 1
    NODE_ENV=DEV \
    SP_FORCE_LOCAL_REST_API=1 \
    SP_FORCE_LOCAL_REST_API_TOKEN="$TOKEN" \
    exec xvfb-run -a npx electron . \
        --custom-url="file://$FRONTEND" \
        --user-data-dir="$PROFILE" \
        --disable-tray \
        --no-sandbox
) > "$LOG" 2>&1 &
SP_PID=$!
echo "==> launched Super Productivity (pid $SP_PID)"

# The API relays every request to the renderer over IPC, so a listening socket
# is not enough: rendererReady has to be true or requests hang and time out.
ready=""
for i in $(seq 1 "$READY_TIMEOUT"); do
    if ! kill -0 "$SP_PID" 2>/dev/null; then
        tail -20 "$LOG" >&2
        fail_env "Super Productivity exited before becoming ready"
    fi
    body="$(curl -s --max-time 2 "$BASE/health" 2>/dev/null)"
    if [ -n "$body" ] && grep -q '"rendererReady":true' <<<"$body"; then
        ready="$i"
        break
    fi
    sleep 1
done
[ -n "$ready" ] || {
    grep -i "local-rest-api" "$LOG" | tail -5 >&2
    fail_env "renderer not ready after ${READY_TIMEOUT}s"
}
echo "==> renderer ready after ${ready}s"
grep -i "local-rest-api" "$LOG" | tail -3 | sed 's/^/    /'

# Confirms this really is a token-enforcing Super Productivity. Without it, a
# run against an older SP would pass while proving nothing about the code path
# that matters.
code="$(curl -s -o /dev/null -w '%{http_code}' "$BASE/tasks")"
if [ "$code" != "401" ]; then
    echo "==> note: unauthenticated /tasks returned $code, not 401;" \
         "this Super Productivity does not enforce a token" >&2
fi
echo "==> unauthenticated /tasks -> $code"

# A fresh profile has no tasks, and the live suite deliberately refuses to pass
# against an empty store. Writes are safe here: this profile is created above
# and thrown away with the runner.
#
# Seeding two bare titles is not enough. The suite checks the *type* of every
# field the client depends on, and skips any field absent from the store — so a
# thin store passes while checking almost nothing, which is the vacuous pass
# this job exists to avoid. Each write below exists to populate fields that
# would otherwise go unchecked: subtasks for parentId/subTaskIds, a completed
# task for doneOn, an archived task for the archived pool, and a current task
# for currentTask/currentTaskId.
api() {
    curl -s -X "$1" "$BASE$2" \
        -H "Authorization: Bearer $TOKEN" \
        -H 'Content-Type: application/json' \
        ${3:+-d "$3"}
}

task_id() { python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' 2>/dev/null; }

parent="$(api POST /tasks '{"title":"Upstream live check: parent","notes":"seeded by upstream-live-check.sh","timeEstimate":3600000,"timeSpent":600000}' | task_id)"
[ -n "$parent" ] || fail_env "could not seed a task"
api POST /tasks "{\"title\":\"Upstream live check: subtask\",\"parentId\":\"$parent\"}" >/dev/null

api POST /tasks '{"title":"Upstream live check: completed","isDone":true}' >/dev/null

archived="$(api POST /tasks '{"title":"Upstream live check: archived"}' | task_id)"
[ -n "$archived" ] || fail_env "could not seed the task to archive"
api POST "/tasks/$archived/archive" >/dev/null

# Leaves currentTask and currentTaskId non-null, which are otherwise null
# throughout a fresh store and so never type-checked.
api POST "/tasks/$parent/start" >/dev/null

echo "==> seeded the throwaway profile"

echo "==> running the live suite"
(
    cd "$REPO_ROOT" || exit 1
    SP_API_TOKEN="$TOKEN" go test -tags live ./... -run TestLive -count=1 -v
)
rc=$?
echo "==> live suite exit=$rc"
[ "$rc" -eq 0 ] || exit 1
