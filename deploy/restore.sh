#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly database="retrotide"
readonly dump_name="retrotide.dump"
readonly tag="retrotide"
readonly ready_attempts=120

started_at="$(date +%s)"
work_dir=""
container=""
container_created=0

fail() {
    echo "restore: $1" >&2
    exit 1
}

elapsed() {
    local seconds
    seconds=$(( $(date +%s) - started_at ))

    printf '%dh %02dm %02ds' $(( seconds / 3600 )) $(( seconds % 3600 / 60 )) $(( seconds % 60 ))
}

cleanup() {
    local status=$?
    local duration

    if [ "$container_created" -eq 1 ]; then
        if ! docker rm --force --volumes "$container" > /dev/null; then
            echo "restore: could not remove the container; remove it with: docker rm --force --volumes ${container}" >&2
            status=1
        fi
    fi

    if [ -n "$work_dir" ]; then
        if ! rm -rf "$work_dir"; then
            echo "restore: could not remove ${work_dir}" >&2
            status=1
        fi
    fi

    if [ "$status" -ne 0 ]; then
        duration="$(elapsed)"
        echo "restore: failed after ${duration}" >&2
    fi

    exit "$status"
}

if [ "$#" -gt 1 ]; then
    echo "usage: bash restore.sh [snapshot id]" >&2
    exit 64
fi

snapshot="${1:-latest}"

if [[ "$snapshot" == -* ]]; then
    fail "the snapshot id must not start with a dash"
fi

for name in RESTIC_REPOSITORY RESTIC_PASSWORD AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
    if [ -z "${!name:-}" ]; then
        fail "${name} is not set; export it from the password manager first"
    fi
done

for tool in docker restic; do
    if ! command -v "$tool" > /dev/null; then
        fail "${tool} is not installed or not on PATH"
    fi
done

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/retrotide-restore.XXXXXXXX")"
restore_dir="${work_dir}/restore"
dump_file="${restore_dir}/${dump_name}"
container="retrotide-restore-$(date -u +%Y%m%d%H%M%S)-$$"

restore_args=(restore "$snapshot" --target "$restore_dir")
if [ "$snapshot" = "latest" ]; then
    restore_args+=(--tag "$tag")
fi

echo "restore: restoring snapshot ${snapshot} into a temporary directory"
restic "${restore_args[@]}"

if [ ! -s "$dump_file" ]; then
    fail "the snapshot holds no ${dump_name}"
fi

pg_password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"

echo "restore: starting the throwaway container ${container}"
POSTGRES_PASSWORD="$pg_password" docker create --name "$container" --network none --env POSTGRES_PASSWORD postgres:18 > /dev/null
container_created=1
docker start "$container" > /dev/null

ready=0
for (( attempt = 1; attempt <= ready_attempts; attempt++ )); do
    if docker exec "$container" pg_isready --host=127.0.0.1 --port=5432 --username=postgres --quiet; then
        ready=1
        break
    fi
    sleep 1
done

if [ "$ready" -ne 1 ]; then
    docker logs --tail 50 "$container" >&2
    fail "the throwaway postgres was not ready after ${ready_attempts} seconds"
fi

echo "restore: creating the roles and the ${database} database"
docker exec "$container" psql --username=postgres --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
    --command="CREATE ROLE retrotide NOLOGIN" \
    --command="CREATE ROLE retrotide_monitor NOLOGIN" \
    --command="CREATE DATABASE ${database} OWNER retrotide"

echo "restore: loading the dump"
docker exec --interactive "$container" pg_restore --username=postgres --dbname="$database" --no-password --exit-on-error < "$dump_file"

echo "restore: running the smoke query"
goose_version="$(docker exec "$container" psql --username=postgres --dbname="$database" --no-psqlrc --tuples-only --no-align --set=ON_ERROR_STOP=1 --command="SELECT max(version_id) FROM goose_db_version WHERE is_applied")"

if [[ ! "$goose_version" =~ ^[0-9]+$ ]]; then
    fail "the smoke query found no applied goose migration"
fi

duration="$(elapsed)"
echo "restore: snapshot ${snapshot} restored at goose version ${goose_version} in ${duration}"
