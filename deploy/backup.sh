#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly database="retrotide"
readonly dump_name="retrotide.dump"
readonly secrets_file="/etc/retrotide/backup.env"

work_dir=""
backup_done=0
ping_failed=0

fail() {
    echo "backup: $1" >&2
    exit 1
}

ping_healthcheck() {
    local suffix="$1"
    local message="$2"

    if ! curl --fail --silent --show-error --max-time 10 --retry 5 --output /dev/null --data-raw "$message" "${HEALTHCHECK_URL%/}${suffix}"; then
        echo "backup: the healthcheck ping${suffix:+ to ${suffix}} failed" >&2
        ping_failed=1
    fi
}

finish() {
    local status=$?

    if [ -n "$work_dir" ]; then
        if ! rm -rf "$work_dir"; then
            echo "backup: could not remove ${work_dir}" >&2
            status=1
        fi
    fi

    if [ "$status" -ne 0 ] && [ "$backup_done" -eq 0 ]; then
        ping_healthcheck "/fail" "backup failed with exit status ${status}"
    fi

    exit "$status"
}

if [ "$#" -ne 1 ]; then
    echo "usage: bash backup.sh <stack dir>" >&2
    exit 64
fi

if [ -z "${HEALTHCHECK_URL:-}" ]; then
    fail "HEALTHCHECK_URL is not set; the service loads it from ${secrets_file}"
fi

trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

ping_healthcheck "/start" "backup started"

for name in RESTIC_REPOSITORY RESTIC_PASSWORD AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
    if [ -z "${!name:-}" ]; then
        fail "${name} is not set; the service loads it from ${secrets_file}"
    fi
done

for tool in docker restic curl; do
    if ! command -v "$tool" > /dev/null; then
        fail "${tool} is not installed or not on PATH"
    fi
done

stack_dir="${1%/}"
env_file="${stack_dir}/.env"

if [ ! -d "$stack_dir" ]; then
    fail "the stack directory ${stack_dir} does not exist"
fi

if [ ! -f "$env_file" ]; then
    fail "${env_file} does not exist"
fi

stack=""
while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"
    if [[ "$line" == STACK=* ]]; then
        stack="${line#STACK=}"
    fi
done < "$env_file"

case "$stack" in
    \"*\" | \'*\')
        stack="${stack:1:${#stack}-2}"
        ;;
esac

if [ -z "$stack" ]; then
    stack="retrotide"
fi

if [[ ! "$stack" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
    fail "STACK in ${env_file} is not a valid compose project name: ${stack}"
fi

container="${stack}-postgres"

if ! running="$(docker inspect --format '{{.State.Running}}' "$container")"; then
    fail "could not inspect the container ${container}"
fi

if [ "$running" != "true" ]; then
    fail "the container ${container} is not running"
fi

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/retrotide-backup.XXXXXXXX")"
dump_file="${work_dir}/${dump_name}"

echo "backup: dumping the ${database} database from ${container}"
docker exec "$container" pg_dump --format=custom --compress=0 --no-password --username=postgres --dbname="$database" > "$dump_file"

if [ ! -s "$dump_file" ]; then
    fail "pg_dump wrote an empty dump"
fi

echo "backup: uploading the dump to the restic repository"
restic backup --host "$stack" --tag "$stack" --stdin --stdin-filename "$dump_name" < "$dump_file"

echo "backup: applying the retention policy"
restic forget --host "$stack" --tag "$stack" --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune

rm -rf "$work_dir"
work_dir=""
backup_done=1

ping_healthcheck "" "backup finished"

if [ "$ping_failed" -ne 0 ]; then
    fail "the snapshot was saved and pruned, but a healthcheck ping failed"
fi

echo "backup: finished"
