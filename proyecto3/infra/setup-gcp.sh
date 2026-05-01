#!/bin/bash
# =============================================================================
# setup-gcp.sh — Crea toda la infraestructura GCP para M.U.M.N.K8s
# Requiere: gcloud CLI autenticado, proyecto GCP activo.
# =============================================================================
set -euo pipefail

# --------------------------------------------------------------------------
# CONFIGURACIÓN — editar según tu proyecto GCP
# --------------------------------------------------------------------------
PROJECT_ID="${GCP_PROJECT:-tu-proyecto-gcp}"
REGION="${GCP_REGION:-us-central1}"
ZONE="${GCP_ZONE:-us-central1-a}"
CLUSTER_NAME="mumk8s-cluster"
ZOT_VM_NAME="zot-registry"
NETWORK_NAME="mumk8s-network"

echo "==================================================================="
echo " M.U.M.N.K8s — Infraestructura GCP"
echo " Proyecto : ${PROJECT_ID}"
echo " Región   : ${REGION}"
echo " Zona     : ${ZONE}"
echo "==================================================================="

# --------------------------------------------------------------------------
# 0. Configurar proyecto activo
# --------------------------------------------------------------------------
gcloud config set project "${PROJECT_ID}"
gcloud config set compute/region "${REGION}"
gcloud config set compute/zone "${ZONE}"

# Habilitar APIs necesarias
echo "==> Habilitando APIs de GCP..."
gcloud services enable \
  container.googleapis.com \
  compute.googleapis.com \
  artifactregistry.googleapis.com \
  --quiet

# --------------------------------------------------------------------------
# 1. Red VPC dedicada
# --------------------------------------------------------------------------
echo "==> Creando VPC ${NETWORK_NAME}..."
gcloud compute networks create "${NETWORK_NAME}" \
  --subnet-mode=auto \
  --quiet 2>/dev/null || echo "  (ya existe)"

# --------------------------------------------------------------------------
# 2. VM externa N1 para Zot (fuera del clúster GKE)
# --------------------------------------------------------------------------
echo "==> Creando VM N1 para Zot registry: ${ZOT_VM_NAME}..."
gcloud compute instances create "${ZOT_VM_NAME}" \
  --machine-type=n1-standard-1 \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --boot-disk-size=50GB \
  --boot-disk-type=pd-standard \
  --network="${NETWORK_NAME}" \
  --zone="${ZONE}" \
  --tags=zot-registry \
  --quiet 2>/dev/null || echo "  (ya existe)"

# Regla de firewall para Zot (puerto 5000)
echo "==> Regla de firewall para Zot (puerto 5000)..."
gcloud compute firewall-rules create allow-zot \
  --allow tcp:5000 \
  --target-tags zot-registry \
  --source-ranges 0.0.0.0/0 \
  --description "Zot OCI Registry" \
  --network="${NETWORK_NAME}" \
  --quiet 2>/dev/null || echo "  (ya existe)"

# Obtener IP de la VM de Zot
ZOT_IP=$(gcloud compute instances describe "${ZOT_VM_NAME}" \
  --zone="${ZONE}" \
  --format="value(networkInterfaces[0].accessConfigs[0].natIP)")
echo "==> Zot VM IP: ${ZOT_IP}"
echo "    Instala Zot en la VM con: scp zot/install-zot.sh zot/config.json ${ZOT_VM_NAME}:~/"
echo "    Luego: gcloud compute ssh ${ZOT_VM_NAME} -- sudo bash install-zot.sh"

# --------------------------------------------------------------------------
# 3. Clúster GKE (N1, con soporte para KubeVirt y Gateway API)
# --------------------------------------------------------------------------
echo "==> Creando clúster GKE ${CLUSTER_NAME} (instancias N1)..."
gcloud container clusters create "${CLUSTER_NAME}" \
  --machine-type=n1-standard-4 \
  --num-nodes=3 \
  --zone="${ZONE}" \
  --network="${NETWORK_NAME}" \
  --enable-ip-alias \
  --gateway-api=standard \
  --release-channel=regular \
  --quiet 2>/dev/null || echo "  (ya existe)"

# Obtener credenciales kubectl
echo "==> Obteniendo credenciales kubectl..."
gcloud container clusters get-credentials "${CLUSTER_NAME}" \
  --zone="${ZONE}"

# --------------------------------------------------------------------------
# 4. Instalar KubeVirt en el clúster
# --------------------------------------------------------------------------
KUBEVIRT_VERSION="v1.2.1"
echo "==> Instalando KubeVirt ${KUBEVIRT_VERSION}..."
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

echo "==> Esperando que KubeVirt esté listo (puede tardar ~3 minutos)..."
kubectl wait --for=condition=Available \
  kubevirt/kubevirt \
  -n kubevirt \
  --timeout=300s

# --------------------------------------------------------------------------
# 5. Instalar virtctl (CLI de KubeVirt)
# --------------------------------------------------------------------------
echo "==> Instalando virtctl..."
wget -q "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/virtctl-${KUBEVIRT_VERSION}-linux-amd64" \
  -O /usr/local/bin/virtctl 2>/dev/null || \
  curl -sLo /usr/local/bin/virtctl \
    "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/virtctl-${KUBEVIRT_VERSION}-linux-amd64"
chmod +x /usr/local/bin/virtctl

# --------------------------------------------------------------------------
# 6. Distribuir el certificado TLS de Zot a los nodos y a Docker
# --------------------------------------------------------------------------
echo "==> Copiando CA cert desde la VM de Zot..."
gcloud compute scp "${ZOT_VM_NAME}:/etc/zot/certs/domain.crt" \
  /tmp/zot-ca.crt --zone="${ZONE}" --quiet

echo "==> Confiando en el cert para Docker (Linux/GKE build node)..."
mkdir -p "/etc/docker/certs.d/${ZOT_IP}:5000"
cp /tmp/zot-ca.crt "/etc/docker/certs.d/${ZOT_IP}:5000/ca.crt"
# No requiere reiniciar Docker; Docker lee certs.d en cada operación.

echo "==> Exportando ZOT_CA_FILE para oras..."
export ZOT_CA_FILE=/tmp/zot-ca.crt

echo ""
echo "IMPORTANTE — Docker Desktop en Windows:"
echo "  1. Copia domain.crt desde la VM:"
echo "     gcloud compute scp ${ZOT_VM_NAME}:/etc/zot/certs/domain.crt ."
echo "  2. Crea el directorio y coloca el cert:"
echo "     New-Item -ItemType Directory -Force \"\$env:USERPROFILE\.docker\certs.d\${ZOT_IP}:5000\""
echo "     Copy-Item domain.crt \"\$env:USERPROFILE\.docker\certs.d\${ZOT_IP}:5000\ca.crt\""
echo "  3. Reinicia Docker Desktop."

# --------------------------------------------------------------------------
# 7. Resumen final
# --------------------------------------------------------------------------
echo ""
echo "==================================================================="
echo " INFRAESTRUCTURA LISTA"
echo "==================================================================="
echo " Zot VM IP     : ${ZOT_IP}:5000"
echo " GKE Cluster   : ${CLUSTER_NAME} (${ZONE})"
echo " KubeVirt      : instalado"
echo " Gateway API   : habilitado (--gateway-api=standard)"
echo ""
echo " Próximos pasos:"
echo " 1. Instalar Zot en la VM: gcloud compute ssh ${ZOT_VM_NAME} -- sudo bash install-zot.sh"
echo " 2. Exportar: export ZOT_HOST=${ZOT_IP}:5000"
echo " 3. Build y push imágenes: bash infra/build-push.sh"
echo " 4. Desplegar: bash infra/deploy.sh"
echo "==================================================================="
