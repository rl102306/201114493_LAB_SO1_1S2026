#!/bin/bash
# =============================================================================
# oci-artifact.sh — Sube y descarga el proto como OCI Artifact en Zot.
#
# El archivo OCI Artifact es: go-grpc-server/proto/war_report.proto
# Este archivo define el contrato gRPC del sistema. Subirlo como OCI Artifact
# garantiza versionado y distribución centralizada del schema gRPC desde el
# mismo registry que las imágenes Docker.
#
# PREREQUISITO: instalar 'oras' CLI
#   https://oras.land/docs/installation
# =============================================================================
set -euo pipefail

ZOT_HOST="${ZOT_HOST:-localhost:5000}"
ZOT_CA_FILE="${ZOT_CA_FILE:-/etc/zot/certs/domain.crt}"
ARTIFACT_REPO="mumk8s/grpc-proto"
ARTIFACT_TAG="latest"
PROTO_FILE="../go-grpc-server/proto/war_report.proto"

# --------------------------------------------------------------------------
# PUSH: subir el .proto como OCI Artifact
# --------------------------------------------------------------------------
push_artifact() {
  echo "==> Subiendo war_report.proto a ${ZOT_HOST}/${ARTIFACT_REPO}:${ARTIFACT_TAG}..."
  oras push \
    --ca-file "${ZOT_CA_FILE}" \
    "${ZOT_HOST}/${ARTIFACT_REPO}:${ARTIFACT_TAG}" \
    "${PROTO_FILE}:application/vnd.mumk8s.proto.v1"
  echo "==> Proto subido exitosamente."
}

# --------------------------------------------------------------------------
# PULL: descargar el .proto desde Zot (usado en CI o en VMs del clúster)
# --------------------------------------------------------------------------
pull_artifact() {
  local output_dir="${1:-./proto-downloaded}"
  mkdir -p "${output_dir}"
  echo "==> Descargando war_report.proto desde ${ZOT_HOST}/${ARTIFACT_REPO}:${ARTIFACT_TAG}..."
  oras pull \
    --ca-file "${ZOT_CA_FILE}" \
    --output "${output_dir}" \
    "${ZOT_HOST}/${ARTIFACT_REPO}:${ARTIFACT_TAG}"
  echo "==> Proto descargado en ${output_dir}/"
}

# --------------------------------------------------------------------------
# PUSH DASHBOARD: también sube el dashboard JSON de Grafana como OCI Artifact
# --------------------------------------------------------------------------
push_dashboard() {
  local DASHBOARD_FILE="../k8s/grafana/dashboard.json"
  local DASH_REPO="mumk8s/grafana-dashboard"
  echo "==> Subiendo dashboard.json a ${ZOT_HOST}/${DASH_REPO}:${ARTIFACT_TAG}..."
  oras push \
    --ca-file "${ZOT_CA_FILE}" \
    "${ZOT_HOST}/${DASH_REPO}:${ARTIFACT_TAG}" \
    "${DASHBOARD_FILE}:application/vnd.mumk8s.grafana-dashboard.v1"
  echo "==> Dashboard subido."
}

# --------------------------------------------------------------------------
# PULL DASHBOARD: descarga el dashboard en la VM de Grafana
# --------------------------------------------------------------------------
pull_dashboard() {
  local output_dir="${1:-/tmp}"
  echo "==> Descargando dashboard.json en ${output_dir}..."
  oras pull \
    --ca-file "${ZOT_CA_FILE}" \
    --output "${output_dir}" \
    "${ZOT_HOST}/mumk8s/grafana-dashboard:${ARTIFACT_TAG}"
  echo "==> Dashboard disponible en ${output_dir}/dashboard.json"
}

# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------
case "${1:-help}" in
  push)       push_artifact ;;
  pull)       pull_artifact "${2:-./proto-downloaded}" ;;
  push-dash)  push_dashboard ;;
  pull-dash)  pull_dashboard "${2:-/tmp}" ;;
  all)
    push_artifact
    push_dashboard
    ;;
  *)
    echo "Uso: $0 {push|pull [dir]|push-dash|pull-dash [dir]|all}"
    echo ""
    echo "  push        Sube war_report.proto como OCI Artifact"
    echo "  pull [dir]  Descarga war_report.proto"
    echo "  push-dash   Sube dashboard.json como OCI Artifact"
    echo "  pull-dash   Descarga dashboard.json (usa en VM Grafana)"
    echo "  all         push + push-dash"
    echo ""
    echo "Variables de entorno:"
    echo "  ZOT_HOST    Host:port del registry (default: localhost:5000)"
    ;;
esac
