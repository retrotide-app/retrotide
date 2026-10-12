#!/usr/bin/env bash
set -euo pipefail
shopt -s inherit_errexit

readonly keep_releases=5
readonly keep_dumps=5
readonly dump_max_age_days=180
readonly health_timeout_seconds=120
readonly wait_timeout_seconds=180
readonly runbook="the deployment runbook"
readonly app_image="ghcr.io/retrotide-app/retrotide"
readonly deploy_lock="/run/lock/retrotide-deploy.lock"

fail() {
    echo "deploy: $*" >&2
    exit 1
}

env_value() {
    local value

    value="$(ENV_KEY="$1" awk 'index($0, ENVIRON["ENV_KEY"] "=") == 1 { value = substr($0, length(ENVIRON["ENV_KEY"]) + 2) } END { print value }' "$env_file")"

    value="${value%$'\r'}"
    case "$value" in
        \"*\" | \'*\')
            value="${value:1:${#value}-2}"
            ;;
    esac

    printf '%s' "$value"
}

write_image_tag() {
    local tmp

    tmp="$(mktemp "$env_file.XXXXXX")"

    if ! IMAGE_TAG_VALUE="$1" awk '
        /^IMAGE_TAG=/ {
            if (!written && ENVIRON["IMAGE_TAG_VALUE"] != "") {
                print "IMAGE_TAG=" ENVIRON["IMAGE_TAG_VALUE"]
                written = 1
            }
            next
        }
        { print }
        END {
            if (!written && ENVIRON["IMAGE_TAG_VALUE"] != "") {
                print "IMAGE_TAG=" ENVIRON["IMAGE_TAG_VALUE"]
            }
        }
    ' "$env_file" > "$tmp"; then
        rm -f "$tmp"
        fail "could not rewrite IMAGE_TAG in $env_file"
    fi

    chmod --reference="$env_file" "$tmp"
    mv "$tmp" "$env_file"
}

prune_to_newest() {
    local keep="$1"
    local dir="$2"
    shift 2
    local stale_list
    local stale

    stale_list="$(find "$dir" -mindepth 1 -maxdepth 1 "$@" -printf '%T@ %p\n' | LC_ALL=C sort -rn | tail -n "+$((keep + 1))" | cut -d ' ' -f 2-)"

    while IFS= read -r stale; do
        if [ -n "$stale" ]; then
            echo "    pruning $stale"
            rm -rf -- "$stale"
        fi
    done <<< "$stale_list"
}

install_init_script() {
    local target="$stack_dir/deploy/postgres/initdb"

    if [ -d "$target" ]; then
        return 0
    fi

    echo "    installing the init script from releases/$sha"
    mkdir -p "$stack_dir/deploy/postgres"
    cp -r "$release_dir/deploy/postgres/initdb" "$target"
}

remove_unused_app_images() {
    local releases
    local containers
    local tags
    local line
    local -A keep=()

    releases="$(find "$stack_dir/releases" -mindepth 1 -maxdepth 1 -type d -printf '%f\n')"
    containers="$(docker ps --all --format '{{.Image}}')"
    tags="$(docker image ls "$app_image" --format '{{.Tag}}')"

    while IFS= read -r line; do
        if [ -n "$line" ]; then
            keep["$line"]=1
        fi
    done <<< "$releases"

    while IFS= read -r line; do
        if [[ "$line" == "$app_image:"* ]]; then
            keep["${line#"$app_image:"}"]=1
        fi
    done <<< "$containers"

    while IFS= read -r line; do
        if [ -z "$line" ] || [ "$line" = "<none>" ] || [ -n "${keep[$line]:-}" ]; then
            continue
        fi

        echo "    removing $app_image:$line"
        if ! docker image rm "$app_image:$line" > /dev/null; then
            fail "$sha is live, but $app_image:$line could not be removed; see $runbook"
        fi
    done <<< "$tags"
}

wait_for_health() {
    local deadline=$((SECONDS + health_timeout_seconds))
    local status="000"

    while [ "$SECONDS" -lt "$deadline" ]; do
        if status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 5 "$health_url")" && [ "$status" = "200" ]; then
            echo "    $health_url answered 200"
            return 0
        fi
        sleep 2
    done

    echo "deploy: $health_url last answered $status (000 means no response)" >&2
    return 1
}

roll_back() {
    local reason="$1"
    local rollback_file="$stack_dir/releases/$previous_tag/docker-compose.prod.yml"

    echo "deploy: $reason" >&2

    if [ -z "$previous_tag" ]; then
        fail "$env_file had no IMAGE_TAG before this deploy, so there is nothing to roll back to and the stack stays on $sha; see $runbook"
    fi

    if [ ! -f "$rollback_file" ]; then
        echo "    releases/$previous_tag is gone, so the rollback uses the compose file of releases/$sha"
        rollback_file="$compose_file"
    fi

    echo "==> rolling back to $previous_tag"
    write_image_tag "$previous_tag"

    if ! docker compose --project-directory "$stack_dir" -f "$rollback_file" up -d --no-deps --wait --wait-timeout "$wait_timeout_seconds" app; then
        fail "the rollback to $previous_tag did not become healthy either, so the stack needs a hand; see $runbook"
    fi

    if ! wait_for_health; then
        fail "the app is back on $previous_tag but /health still fails; see $runbook"
    fi

    fail "deploying $sha failed, and the app is back on $previous_tag"
}

if [ "$#" -ne 2 ]; then
    fail "usage: bash <stack dir>/releases/<sha>/deploy/deploy.sh <stack dir> <sha>"
fi

stack_dir="${1%/}"
sha="$2"

if [[ "$stack_dir" != /* ]]; then
    fail "the stack directory must be an absolute path, not '$stack_dir'"
fi

if [[ ! "$sha" =~ ^[0-9a-f]{40}$ ]]; then
    fail "the release must be a full 40-character commit SHA, not '$sha'"
fi

env_file="$stack_dir/.env"
release_dir="$stack_dir/releases/$sha"
compose_file="$release_dir/docker-compose.prod.yml"
backups_dir="$stack_dir/backups"
dump_file="$backups_dir/predeploy-$sha-$(date -u +%Y%m%dT%H%M%SZ).dump"

for required in "$env_file" "$stack_dir/postgres.env" "$compose_file"; do
    if [ ! -f "$required" ] || [ ! -r "$required" ]; then
        fail "$required is missing or unreadable; set the stack up as $runbook describes"
    fi
done

for tool in docker curl flock; do
    if ! command -v "$tool" > /dev/null; then
        fail "$tool is not installed on this host"
    fi
done

stack="$(env_value STACK)"
app_port="$(env_value APP_PORT)"
previous_tag="$(env_value IMAGE_TAG)"

if [ -z "$stack" ]; then
    fail "STACK is missing from $env_file; every stack's .env must set it, see $runbook"
fi

if [ -z "$app_port" ]; then
    fail "APP_PORT is missing from $env_file; every stack's .env must set it, see $runbook"
fi

if [[ ! "$stack" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
    fail "STACK in $env_file is not a valid compose project name"
fi

if [[ ! "$app_port" =~ ^[0-9]{1,5}$ ]]; then
    fail "APP_PORT in $env_file is not a port number"
fi

if [ -n "$previous_tag" ] && [[ ! "$previous_tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
    fail "IMAGE_TAG in $env_file is not a valid image tag"
fi

unset IMAGE_TAG STACK APP_PORT APP_CPUS APP_MEM_LIMIT APP_GOMEMLIMIT POSTGRES_CPUS POSTGRES_MEM_LIMIT

postgres_container="$stack-postgres"
health_url="http://127.0.0.1:$app_port/health"
compose=(docker compose --project-directory "$stack_dir" -f "$compose_file")

echo "==> $stack: deploying $sha over ${previous_tag:-no recorded IMAGE_TAG}"
touch "$release_dir"

exec 9>> "$deploy_lock"
flock --shared 9

postgres_state="$(docker ps --all --filter "name=$postgres_container" --format '{{.Names}} {{.State}}' | awk -v name="$postgres_container" '$1 == name { print $2 }')"
if [ "$postgres_state" != "running" ]; then
    postgres_volume="${stack}_postgres-data"
    volume_found="$(docker volume ls --format '{{.Name}}' | awk -v name="$postgres_volume" '$1 == name { print "yes" }')"

    if [ -n "$previous_tag" ] && [ -z "$volume_found" ]; then
        fail "$env_file records IMAGE_TAG=$previous_tag, but the $postgres_volume volume is gone, so starting Postgres would create an empty database; restore it as $runbook describes, or remove IMAGE_TAG from $env_file to start this stack empty"
    fi

    echo "==> $postgres_container is ${postgres_state:-missing}, so the data services start first"
    install_init_script

    if ! IMAGE_TAG="${previous_tag:-$sha}" "${compose[@]}" up -d --wait --wait-timeout "$wait_timeout_seconds" postgres valkey postgres-exporter; then
        fail "Postgres, Valkey or the exporter did not become healthy, so nothing was deployed; see $runbook"
    fi
fi

echo "==> step 2: pg_dump of the retrotide database into $dump_file"
(umask 077 && mkdir -p "$backups_dir")
partial_dump="$dump_file.partial"
if ! (umask 077 && docker exec "$postgres_container" sh -c "exec pg_dump -Fc -U \"\$POSTGRES_USER\" -d retrotide" > "$partial_dump"); then
    rm -f "$partial_dump"
    fail "pg_dump failed, so nothing was deployed"
fi
mv "$partial_dump" "$dump_file"
prune_to_newest "$keep_dumps" "$backups_dir" -type f -name 'predeploy-*.dump'
if ! find "$backups_dir" -mindepth 1 -maxdepth 1 -type f -name 'predeploy-*.dump' -mtime "+$dump_max_age_days" -printf '    pruning %p, older than 6 months\n' -delete; then
    fail "could not delete the pre-deploy dumps older than 6 months from $backups_dir, so nothing was deployed"
fi

echo "==> step 3: IMAGE_TAG=$sha in $env_file"
write_image_tag "$sha"

echo "==> step 4: pulling and recreating the app"
if ! "${compose[@]}" pull app; then
    write_image_tag "$previous_tag"
    fail "pulling the $sha image failed; IMAGE_TAG is back to ${previous_tag:-unset} and the running app was not touched"
fi

if ! "${compose[@]}" up -d --no-deps --wait --wait-timeout "$wait_timeout_seconds" app; then
    roll_back "the $sha app container did not become healthy"
fi

echo "==> step 5: waiting up to ${health_timeout_seconds}s for $health_url"
if ! wait_for_health; then
    roll_back "the $sha app did not pass the /health gate"
fi

echo "==> step 6: pruning old releases and the images nothing uses"
prune_to_newest "$keep_releases" "$stack_dir/releases" -type d

lock_status=0
flock --exclusive --nonblock --conflict-exit-code 75 9 || lock_status=$?
if [ "$lock_status" -eq 75 ]; then
    echo "    another deploy is in flight on this host, so the unused app images wait for the next deploy"
elif [ "$lock_status" -ne 0 ]; then
    fail "$sha is live, but $deploy_lock could not be locked; see $runbook"
else
    remove_unused_app_images
fi

docker image prune -f

echo "==> $stack is on $sha"
