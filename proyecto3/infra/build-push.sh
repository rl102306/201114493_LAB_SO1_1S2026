#!/bin/bash
# =============================================================================
# build-push.sh — Construye y sube todas las imágenes Docker a Zot.
# Requiere: ZOT_HOST exportado, Docker corriendo.
# =============================================================================
set -euo pipefail

: "${ZOT_HOST:?ERROR: Exporta ZOT_HOST=<ip-zot>:5000}"

TAG="${IMAGE_TAG:-latest}"
BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"

build_push() {
  local name="$1"
  local context="$2"
  local image="${ZOT_HOST}/mumk8s/${name}:${TAG}"
  echo "==> Building ${image}..."
  docker build -t "${image}" "${BASE_DIR}/${context}"
  echo "==> Pushing ${image}..."
  docker push "${image}"
  echo "    OK: ${image}"
}

echo "==================================================================="
echo " M.U.M.N.K8s — Build & Push | ZOT=${ZOT_HOST} | TAG=${TAG}"
echo "==================================================================="

build_push "rust-api"            "rust-api"
build_push "go-grpc-client"      "go-grpc-client"
build_push "go-grpc-server"      "go-grpc-server"
build_push "go-rabbitmq-writer"  "go-rabbitmq-writer"
build_push "go-consumer"         "go-consumer"
build_push "locust"              "locust"

echo ""
echo "==> Subiendo OCI Artifacts (proto + dashboard)..."
export ZOT_HOST
bash "${BASE_DIR}/zot/oci-artifact.sh" all

echo ""
echo "==================================================================="
echo " Todas las imágenes subidas a ${ZOT_HOST}/mumk8s/"
echo "==================================================================="
