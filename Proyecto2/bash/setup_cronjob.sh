#!/bin/bash
# Script para configurar el cronjob
# Autor: VGOMEZ - 201114493
# Proyecto 2 SO1

SCRIPT_PATH="/home/ronaldolara/201114493_LAB_SO1_1S2026/Proyecto2/bash/generate_containers.sh"

# Agregar cronjob cada 2 minutos
(crontab -l 2>/dev/null; echo "*/2 * * * * $SCRIPT_PATH >> /var/log/containers.log 2>&1") | crontab -

echo "Cronjob configurado exitosamente"
crontab -l
