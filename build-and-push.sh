#!/bin/bash
# Build and push tas-agent-builder, tagged from VERSION (CLAUDE.md versioning
# policy). It used to push only :latest, which is how production came to run
# hand-named tags (spaceiso-892ff8f, model-fallback-...) with no revision on
# them. Build from a clean checkout of a merged commit: the revision label is
# that commit, and the build refuses to run with uncommitted changes.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REGISTRY="registry-api.tas.scharber.com"
IMAGE="${REGISTRY}/tas-agent-builder"
VERSION="$(tr -d '[:space:]' < "${SCRIPT_DIR}/VERSION")"
REVISION="$(git -C "${SCRIPT_DIR}" rev-parse HEAD)"

if [ -n "$(git -C "${SCRIPT_DIR}" status --porcelain --untracked-files=no)" ]; then
  echo "refusing to build: uncommitted changes (the revision label would lie)" >&2
  exit 1
fi

echo "=== Building tas-agent-builder ${VERSION} (${REVISION}) ==="

# Copy shared go-events module into build context
rm -rf "${SCRIPT_DIR}/go-events"
cp -r "${SCRIPT_DIR}/../aether-shared/go-events" "${SCRIPT_DIR}/go-events"
trap 'rm -rf "${SCRIPT_DIR}/go-events"' EXIT

docker build \
  --label org.opencontainers.image.version="${VERSION}" \
  --label org.opencontainers.image.revision="${REVISION}" \
  --label org.opencontainers.image.created="$(date -u +%FT%TZ)" \
  --label tas.variant= \
  --label tas.config.sha256="$(printf '' | sha256sum | cut -d' ' -f1)" \
  -t "${IMAGE}:${VERSION}" "${SCRIPT_DIR}"

docker push "${IMAGE}:${VERSION}"
echo "=== Done: ${IMAGE}:${VERSION} ==="
