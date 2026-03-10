#!/bin/bash
# Script para generar contenedores aleatorios
# Autor: VGOMEZ - 201114493
# Proyecto 2 SO1

IMAGENES=("roldyoran/go-client" "alpine_cpu" "alpine_sleep")

for i in {1..5}; do
    RANDOM_INDEX=$((RANDOM % 3))
    IMAGEN=${IMAGENES[$RANDOM_INDEX]}

    case $IMAGEN in
        "roldyoran/go-client")
            docker run -d --name "container_ram_$(date +%s)_$i" roldyoran/go-client
            echo "Contenedor alto consumo RAM creado: container_ram_$(date +%s)_$i"
            ;;
        "alpine_cpu")
            docker run -d --name "container_cpu_$(date +%s)_$i" alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done"
            echo "Contenedor alto consumo CPU creado: container_cpu_$(date +%s)_$i"
            ;;
        "alpine_sleep")
            docker run -d --name "container_low_$(date +%s)_$i" alpine sleep 240
            echo "Contenedor bajo consumo creado: container_low_$(date +%s)_$i"
            ;;
    esac

    sleep 1
done

echo "5 contenedores generados exitosamente"
