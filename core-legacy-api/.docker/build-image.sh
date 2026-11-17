#!/usr/bin/env bash
# Build with a temporary Docker daemon for Testcontainers; no host ports exposed.
set -euo pipefail
cd "$(dirname "$0")/.."
docker buildx version >/dev/null
umask 077
state=$(mktemp -d)
daemon="core-legacy-api-tests-$(basename "$state" | tr '[:upper:]' '[:lower:]')"
cleanup() {
    docker rm -fv "$daemon" >/dev/null 2>&1 || true
    rm -rf "$state"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
docker run -d --privileged --name "$daemon" --network bridge \
    -e DOCKER_TLS_CERTDIR=/certs "${TEST_DOCKER_IMAGE:-docker:29.10.0-dind}" >/dev/null
ready=false
for ((attempt=0; attempt<120; attempt++)); do
    if docker exec "$daemon" docker info >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 1
done
if [[ "$ready" != true ]]; then
    docker logs "$daemon" >&2
    exit 1
fi
address=$(docker inspect --format '{{.NetworkSettings.Networks.bridge.IPAddress}}' "$daemon")
test -n "$address"
mkdir "$state/client"
docker cp "$daemon:/certs/client/." "$state/client/"
cat > "$state/client/environment.sh" <<EOF
export DOCKER_HOST=tcp://$address:2376
export DOCKER_TLS_VERIFY=1
export TESTCONTAINERS_HOST_OVERRIDE=$address
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
export NO_PROXY="\${NO_PROXY:-},$address"
export no_proxy="\$NO_PROXY"
EOF
tar -cf "$state/client.tar" -C "$state/client" ca.pem cert.pem key.pem environment.sh
docker buildx build --secret "id=test_docker_config,src=$state/client.tar" "$@" .
