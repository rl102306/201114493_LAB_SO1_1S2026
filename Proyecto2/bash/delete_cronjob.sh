#!/bin/bash
# Script para eliminar el cronjob
# Autor: VGOMEZ - 201114493

crontab -l | grep -v "generate_containers.sh" | crontab -
echo "Cronjob eliminado exitosamente"
