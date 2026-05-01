#!/bin/bash
# =============================================================================
# install-zot.sh — Instala y configura Zot registry en VM externa GCP (N1)
# Ejecutar como root o con sudo en la VM externa al clúster GKE.
# =============================================================================
set -euo pipefail

ZOT_VERSION="2.0.3"
ZOT_PORT="5000"
ZOT_DATA_DIR="/var/lib/registry"
ZOT_BIN="/usr/local/bin/zot"

echo "==> Instalando Zot v${ZOT_VERSION}..."
wget -q "https://github.com/project-zot/zot/releases/download/v${ZOT_VERSION}/zot-linux-amd64" \
  -O "${ZOT_BIN}"
chmod +x "${ZOT_BIN}"

echo "==> Creando directorios..."
mkdir -p "${ZOT_DATA_DIR}"
mkdir -p /etc/zot/certs

echo "==> Generando certificado TLS self-signed..."
ZOT_IP=$(hostname -I | awk '{print $1}')
EXTERNAL_IP=$(curl -sf \
  "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip" \
  -H "Metadata-Flavor: Google" 2>/dev/null || echo "")

SAN="IP:${ZOT_IP}"
if [ -n "${EXTERNAL_IP}" ]; then
  SAN="${SAN},IP:${EXTERNAL_IP}"
  echo "    IPs en SAN: ${ZOT_IP} (interna) + ${EXTERNAL_IP} (externa)"
fi

openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
  -keyout /etc/zot/certs/domain.key \
  -out    /etc/zot/certs/domain.crt \
  -subj   "/CN=${ZOT_IP}" \
  -addext "subjectAltName=${SAN}"
chmod 600 /etc/zot/certs/domain.key

echo "==> Copiando configuración..."
cp "$(dirname "$0")/config.json" /etc/zot/config.json

echo "==> Creando servicio systemd..."
cat > /etc/systemd/system/zot.service << EOF
[Unit]
Description=Zot OCI Container Registry
After=network.target

[Service]
ExecStart=${ZOT_BIN} serve /etc/zot/config.json
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable zot
systemctl start zot

echo "==> Zot corriendo en https://${ZOT_IP}:${ZOT_PORT}"
echo "==> UI disponible en https://${ZOT_IP}:${ZOT_PORT}/v2/_catalog"

echo ""
echo "=== CA CERTIFICATE (distribuir a todos los clientes) ========================"
cat /etc/zot/certs/domain.crt
echo "============================================================================="
echo ""
echo "IMPORTANTE — Para que los clientes confíen en este registry:"
echo ""
echo "  Linux / GKE nodes (containerd):"
echo "    mkdir -p /etc/containerd/certs.d/${ZOT_IP}:${ZOT_PORT}"
echo "    scp <zot-vm>:/etc/zot/certs/domain.crt /etc/containerd/certs.d/${ZOT_IP}:${ZOT_PORT}/ca.crt"
echo ""
echo "  Docker en Linux:"
echo "    mkdir -p /etc/docker/certs.d/${ZOT_IP}:${ZOT_PORT}"
echo "    scp <zot-vm>:/etc/zot/certs/domain.crt /etc/docker/certs.d/${ZOT_IP}:${ZOT_PORT}/ca.crt"
echo ""
echo "  Docker Desktop en Windows:"
echo "    mkdir \$env:USERPROFILE\.docker\certs.d\${ZOT_IP}:${ZOT_PORT}"
echo "    # Copiar domain.crt como ca.crt en ese directorio, luego reiniciar Docker Desktop"
echo ""
echo "  oras CLI:"
echo "    export ZOT_CA_FILE=/etc/zot/certs/domain.crt"
echo ""
echo "NOTA: Ejecuta esto desde tu máquina para abrir el puerto en GCP firewall:"
echo "  gcloud compute firewall-rules create allow-zot \\"
echo "    --allow tcp:${ZOT_PORT} \\"
echo "    --source-ranges 0.0.0.0/0 \\"
echo "    --description 'Zot OCI Registry (HTTPS)'"
