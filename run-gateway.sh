#!/usr/bin/env bash
# Runs the published refract gateway image in the foreground, polling a config
# server over HTTP. This is the "Start the gateway" step of the User Guide's
# Quickstart. It works even if the config server isn't up yet - the gateway
# serves a built-in default configuration until it is.
#
# Override any of these through the environment, e.g.:
#   IMAGE_TAG=v1.0.0 CONFIG_SOURCE_URL=http://my-config-server:8090 ./run-gateway.sh
set -euo pipefail

IMAGE="${IMAGE:-jecklgamis/refract}"
IMAGE_TAG="${IMAGE_TAG:-main}"
# The published images are linux/amd64 only for now, so request that explicitly -
# on Apple Silicon (arm64) Docker then runs them under emulation instead of failing
# with "no matching manifest for linux/arm64/v8". Set PLATFORM="" to use the host
# platform once multi-arch images are published.
PLATFORM="${PLATFORM-linux/amd64}"
PLATFORM_ARGS=()
[ -n "${PLATFORM}" ] && PLATFORM_ARGS=(--platform "${PLATFORM}")
CONTAINER_NAME="${CONTAINER_NAME:-refract}"
PROXY_PORT="${PROXY_PORT:-8080}"
ADMIN_PORT="${ADMIN_PORT:-9901}"
CONFIG_SOURCE_URL="${CONFIG_SOURCE_URL:-http://host.docker.internal:8090}"
CONFIG_API_TOKEN="${CONFIG_API_TOKEN:-default}" # fine for localhost only

docker pull ${PLATFORM_ARGS[@]+"${PLATFORM_ARGS[@]}"} "${IMAGE}:${IMAGE_TAG}"
docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true
exec docker run ${PLATFORM_ARGS[@]+"${PLATFORM_ARGS[@]}"} --name "${CONTAINER_NAME}" \
  -p "${PROXY_PORT}:8080" -p "${ADMIN_PORT}:9901" \
  -e CONFIG_SOURCE_KIND=http \
  -e CONFIG_SOURCE_URL="${CONFIG_SOURCE_URL}" \
  -e CONFIG_API_TOKEN="${CONFIG_API_TOKEN}" \
  "${IMAGE}:${IMAGE_TAG}"
