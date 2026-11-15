#!/usr/bin/env bash
# Runs the published refract config server image in the foreground. This is the
# "Start the config server" step of the User Guide's Quickstart: the gateway
# polls it for CDS/LDS/runtime config, and `refractctl push-http` uploads to it.
#
# Override any of these through the environment, e.g.:
#   IMAGE_TAG=v1.0.0 API_TOKEN=s3cret ./run-config-server.sh
set -euo pipefail

IMAGE="${IMAGE:-jecklgamis/refract-config-server}"
IMAGE_TAG="${IMAGE_TAG:-main}"
# The published images are linux/amd64 only for now, so request that explicitly -
# on Apple Silicon (arm64) Docker then runs them under emulation instead of failing
# with "no matching manifest for linux/arm64/v8". Set PLATFORM="" to use the host
# platform once multi-arch images are published.
PLATFORM="${PLATFORM-linux/amd64}"
PLATFORM_ARGS=()
[ -n "${PLATFORM}" ] && PLATFORM_ARGS=(--platform "${PLATFORM}")
CONTAINER_NAME="${CONTAINER_NAME:-refract-config-server}"
PORT="${PORT:-8090}"
API_TOKEN="${API_TOKEN:-default}" # fine for localhost only

docker pull ${PLATFORM_ARGS[@]+"${PLATFORM_ARGS[@]}"} "${IMAGE}:${IMAGE_TAG}"
docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true
exec docker run ${PLATFORM_ARGS[@]+"${PLATFORM_ARGS[@]}"} --name "${CONTAINER_NAME}" \
  -p "${PORT}:8090" \
  -e API_TOKEN="${API_TOKEN}" \
  "${IMAGE}:${IMAGE_TAG}"
