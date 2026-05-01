#!/bin/bash
# =============================================================================
# deploy.sh — Despliega todo el stack M.U.M.N.K8s en GKE.
# Requiere: kubectl configurado, ZOT_HOST exportado.
# =============================================================================
set -euo pipefail

: "${ZOT_HOST:?ERROR: Exporta ZOT_HOST=<ip-zot>:5000}"

BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
K8S="${BASE_DIR}/k8s"

echo "==> Sustituyendo ZOT_HOST=${ZOT_HOST} en deployments..."
# Crear copia temporal con la variable reemplazada
TMP_DIR=$(mktemp -d)
cp -r "${K8S}" "${TMP_DIR}/"
find "${TMP_DIR}/k8s/deployments" -name "*.yaml" -exec \
  sed -i "s|\${ZOT_HOST}|${ZOT_HOST}|g" {} \;

echo "==> [1/8] Namespace..."
kubectl apply -f "${K8S}/namespace.yaml"

echo "==> [2/8] ConfigMaps..."
kubectl apply -f "${K8S}/configmaps/"

echo "==> [3/8] KubeVirt VMs (Valkey + Grafana)..."
kubectl apply -f "${K8S}/kubevirt/valkey-vm.yaml"
kubectl apply -f "${K8S}/kubevirt/grafana-vm.yaml"
echo "     (Las VMs tardan ~5 minutos en inicializarse)"

echo "==> [4/8] RabbitMQ..."
kubectl apply -f "${TMP_DIR}/k8s/deployments/rabbitmq.yaml"
kubectl apply -f "${K8S}/services/rabbitmq-svc.yaml"
echo "     Esperando RabbitMQ ready..."
kubectl wait --for=condition=available deployment/rabbitmq \
  -n mumk8s --timeout=180s

echo "==> [5/8] Go services — Deployment 2 y 3..."
kubectl apply -f "${TMP_DIR}/k8s/deployments/go-rabbitmq-writer.yaml"
kubectl apply -f "${K8S}/services/go-rabbitmq-writer-svc.yaml"
kubectl apply -f "${TMP_DIR}/k8s/deployments/go-grpc-server.yaml"
kubectl apply -f "${K8S}/services/go-grpc-server-svc.yaml"

echo "==> [6/8] Go services — Deployment 1 + Consumer..."
kubectl apply -f "${TMP_DIR}/k8s/deployments/go-grpc-client.yaml"
kubectl apply -f "${K8S}/services/go-grpc-client-svc.yaml"
kubectl apply -f "${TMP_DIR}/k8s/deployments/go-consumer.yaml"

echo "==> [7/8] Rust API + HPA..."
kubectl apply -f "${TMP_DIR}/k8s/deployments/rust-api.yaml"
kubectl apply -f "${K8S}/services/rust-api-svc.yaml"
kubectl apply -f "${K8S}/hpa/rust-api-hpa.yaml"

echo "==> [8/8] Gateway API..."
kubectl apply -f "${K8S}/gateway/gateway-class.yaml"
kubectl apply -f "${K8S}/gateway/gateway.yaml"
kubectl apply -f "${K8S}/gateway/http-route.yaml"

# Cleanup temp
rm -rf "${TMP_DIR}"

echo ""
echo "==> Esperando deployments listos..."
for dep in go-rabbitmq-writer go-grpc-server go-grpc-client go-consumer rust-api; do
  kubectl wait --for=condition=available deployment/${dep} \
    -n mumk8s --timeout=120s && echo "  OK: ${dep}" || echo "  WARN: ${dep} no listo"
done

echo ""
echo "==================================================================="
GATEWAY_IP=$(kubectl get gateway mumk8s-gateway -n mumk8s \
  -o jsonpath='{.status.addresses[0].value}' 2>/dev/null || echo "<pendiente>")
GRAFANA_IP=$(kubectl get svc grafana-vm-svc -n mumk8s \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "<pendiente>")

echo " DESPLIEGUE COMPLETO"
echo " Gateway IP   : ${GATEWAY_IP}"
echo " Endpoint     : http://${GATEWAY_IP}/grpc-201114493"
echo " Grafana      : http://${GRAFANA_IP}:3000  (admin/admin123)"
echo ""
echo " Locust:"
echo "   docker run -p 8089:8089 ${ZOT_HOST}/mumk8s/locust:latest --host http://${GATEWAY_IP}"
echo "   Abrir: http://localhost:8089"
echo "==================================================================="
