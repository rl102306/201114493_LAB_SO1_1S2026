# Manual Técnico — M.U.M.N.K8s
## Monitoreo de Unidades Militares en la Nube con Kubernetes

**Universidad San Carlos de Guatemala — Sistemas Operativos 1**
**Carnet:** 201114493 | **País asignado:** RUS (último dígito 3 → RUS)

---

## 1. Arquitectura General

El sistema procesa reportes militares en tiempo real mediante una cadena de microservicios desplegados en GKE sobre GCP.

```
┌──────────────────────────────────────────────────────────────────────┐
│                          INTERNET / LOCUST                           │
└───────────────────────────────┬──────────────────────────────────────┘
                                │ HTTP POST /grpc-201114493
                                ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    GKE — Namespace: mumk8s                           │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │              Kubernetes Gateway API (L7 externo)             │    │
│  └──────────────────────────┬────────────────────────────────── ┘    │
│                             │ HTTPRoute: /grpc-201114493             │
│                             ▼                                        │
│  ┌──────────────────────────────────────────┐                       │
│  │  rust-api  (HPA: 1-3 pods, CPU >30%)     │  Deployment          │
│  │  POST /grpc-201114493 → valida → reenvía  │                       │
│  └──────────────────────┬───────────────────┘                       │
│                         │ HTTP POST /report                          │
│                         ▼                                            │
│  ┌──────────────────────────────────────────┐                       │
│  │  go-grpc-client  (Deployment 1)          │                       │
│  │  Servidor HTTP → cliente gRPC            │                       │
│  └──────────────────────┬───────────────────┘                       │
│                         │ gRPC SendReport                            │
│                         ▼                                            │
│  ┌──────────────────────────────────────────┐                       │
│  │  go-grpc-server  (Deployment 2)          │                       │
│  │  Servidor gRPC → reenvía via HTTP        │                       │
│  └──────────────────────┬───────────────────┘                       │
│                         │ HTTP POST /publish                         │
│                         ▼                                            │
│  ┌──────────────────────────────────────────┐                       │
│  │  go-rabbitmq-writer  (Deployment 3)      │                       │
│  │  Publica en cola war_reports             │                       │
│  └──────────────────────┬───────────────────┘                       │
│                         │ AMQP publish                               │
│                         ▼                                            │
│  ┌──────────────────────────────────────────┐                       │
│  │  RabbitMQ  (cola: war_reports)           │  Deployment          │
│  └──────────────────────┬───────────────────┘                       │
│                         │ AMQP consume                               │
│                         ▼                                            │
│  ┌──────────────────────────────────────────┐                       │
│  │  go-consumer  (Deployment 4)             │                       │
│  │  Procesa y almacena estadísticas         │                       │
│  └──────────────────────┬───────────────────┘                       │
│                         │ Redis protocol                             │
└─────────────────────────┼────────────────────────────────────────────┘
                          │
        ┌─────────────────┼───────────────────┐
        ▼                                     ▼
┌────────────────────┐           ┌─────────────────────┐
│  Valkey (KubeVirt) │           │  Grafana (KubeVirt) │
│  VM 1 — puerto 6379│◄──────────│  VM 2 — puerto 3000  │
└────────────────────┘           └─────────────────────┘

VM EXTERNA GCP:
┌─────────────────────────────────────────┐
│  Zot Registry — puerto 5000             │
│  (fuera del clúster GKE)                │
└─────────────────────────────────────────┘
```

---

## 2. Flujo Completo de Datos

1. **Locust** genera `POST /grpc-201114493` con JSON aleatorio hacia el Gateway externo.
2. **Kubernetes Gateway API** enruta la petición al Service `rust-api-svc:8080`.
3. **rust-api** valida país y rangos, luego hace `POST /report` a `go-grpc-client-svc:8080`.
4. **go-grpc-client** decodifica el JSON y llama al gRPC `SendReport` en `go-grpc-server-svc:50051`.
5. **go-grpc-server** recibe el `WarReportRequest`, lo serializa a JSON y hace `POST /publish` a `go-rabbitmq-writer-svc:8081`.
6. **go-rabbitmq-writer** publica el mensaje en la cola `war_reports` de RabbitMQ.
7. **go-consumer** consume de la cola, procesa el mensaje y escribe estadísticas en Valkey.
8. **Grafana** consulta Valkey via plugin `redis-datasource` y visualiza el dashboard.

---

## 3. Estructura de Mensajes

### JSON (Locust → rust-api)

```json
{
  "country": "RUS",
  "warplanes_in_air": 42,
  "warships_in_water": 14,
  "timestamp": "2026-03-12T20:15:30Z"
}
```

| Campo | Tipo | Rango |
|---|---|---|
| `country` | string | `USA \| RUS \| CHN \| ESP \| GTM` |
| `warplanes_in_air` | int32 | 0 – 50 |
| `warships_in_water` | int32 | 0 – 30 |
| `timestamp` | string ISO8601 | — |

### Proto3 (go-grpc-client → go-grpc-server)

```protobuf
syntax = "proto3";
package wartweets;
option go_package = "./proto";

message WarReportRequest {
  Countries country       = 1;
  int32 warplanes_in_air  = 2;
  int32 warships_in_water = 3;
  string timestamp        = 4;
}

enum Countries {
  countries_unknown = 0;
  usa = 1; rus = 2; chn = 3; esp = 4; gtm = 5;
}

message WarReportResponse { string status = 1; }

service WarReportService {
  rpc SendReport (WarReportRequest) returns (WarReportResponse);
}
```

---

## 4. Configuración de Kubernetes Gateway API

Se utiliza el controlador nativo de GKE en lugar de un Ingress Controller.

### GatewayClass

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: mumk8s-gateway-class
spec:
  controllerName: networking.gke.io/gateway
```

Usa `gke-l7-global-external-managed` que provisiona un Google Cloud Load Balancer externo.

### Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: mumk8s-gateway
  namespace: mumk8s
spec:
  gatewayClassName: gke-l7-global-external-managed
  listeners:
    - name: http
      protocol: HTTP
      port: 80
```

### HTTPRoute

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: war-report-route
  namespace: mumk8s
spec:
  parentRefs:
    - name: mumk8s-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /grpc-201114493
      backendRefs:
        - name: rust-api-svc
          port: 8080
```

Obtener la IP externa del Gateway:
```bash
kubectl get gateway mumk8s-gateway -n mumk8s \
  -o jsonpath='{.status.addresses[0].value}'
```

---

## 5. Comunicación REST y gRPC

### REST (Locust → rust-api → go-grpc-client)

- Protocolo: HTTP/1.1
- Formato: `application/json`
- rust-api valida los campos antes de reenviar
- Timeout configurado: 10 segundos

### gRPC (go-grpc-client → go-grpc-server)

- Protocolo: HTTP/2 (gRPC)
- Transporte: insecure (no TLS, comunicación intra-clúster)
- La definición del servicio es `WarReportService.SendReport`
- El `.proto` se compila en tiempo de build del Dockerfile usando `protoc`

### HTTP interno (go-grpc-server → go-rabbitmq-writer)

- Protocolo: HTTP/1.1
- Endpoint: `POST /publish`
- Timeout: 10 segundos
- Reconexión automática ante fallo de publicación

---

## 6. Uso de RabbitMQ

### Configuración de la cola

| Parámetro | Valor |
|---|---|
| Nombre | `war_reports` |
| Durable | `true` (sobrevive reinicios) |
| Auto-delete | `false` |
| DeliveryMode | `Persistent` |
| QoS (prefetch) | 10 mensajes |

### Flujo go-rabbitmq-writer → RabbitMQ → go-consumer

```
go-rabbitmq-writer
  └── AMQP Publish → exchange="" (default), routing_key="war_reports"
        └── Queue: war_reports (durable)
              └── go-consumer consumes con Ack manual
                    ├── Éxito → d.Ack(false)
                    ├── Error parseo → d.Nack(false, false)  [descarta]
                    └── Error proceso → d.Nack(false, true)  [reencola]
```

### Reconexión automática

Ambos go-rabbitmq-writer y go-consumer implementan lógica de reconexión:
- Hasta 15 intentos con espera de 5s entre cada uno
- go-consumer detecta cierre de conexión via `NotifyClose` y reconecta en el loop externo

---

## 7. Despliegue de Valkey y Grafana en KubeVirt

### Prerrequisito: KubeVirt instalado

```bash
KUBEVIRT_VERSION="v1.2.1"
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml
kubectl wait --for=condition=Available kubevirt/kubevirt -n kubevirt --timeout=300s
```

### Valkey VM (VM 1)

- **Base image:** `quay.io/containerdisks/fedora:39`
- **Inicialización:** cloud-init descarga e instala Valkey 7.2.5, crea servicio systemd
- **Configuración:** bind `0.0.0.0:6379`, persistence habilitada (AOF)
- **Service K8s:** `valkey-vm-svc:6379` (ClusterIP), selector `kubevirt.io/vm: valkey-vm`

```bash
# Verificar VM arriba
kubectl get vm -n mumk8s
kubectl get vmi -n mumk8s

# Conectarse via console
virtctl console valkey-vm -n mumk8s
```

### Grafana VM (VM 2)

- **Base image:** `quay.io/containerdisks/fedora:39`
- **Inicialización:** cloud-init instala Grafana 11.0, plugin `redis-datasource`, provisiona datasource Valkey automáticamente
- **Service K8s:** `grafana-vm-svc:3000` (LoadBalancer — IP externa)
- **Credenciales:** `admin / admin123`

#### Importar dashboard manualmente (primera vez)

```bash
GRAFANA_IP=$(kubectl get svc grafana-vm-svc -n mumk8s \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

curl -s -X POST \
  "http://admin:admin123@${GRAFANA_IP}:3000/api/dashboards/import" \
  -H "Content-Type: application/json" \
  -d "{\"dashboard\": $(cat k8s/grafana/dashboard.json), \"overwrite\": true, \"folderId\": 0}"
```

---

## 8. Configuración del HPA

El HPA escala automáticamente `rust-api` según CPU.

```yaml
spec:
  scaleTargetRef:
    kind: Deployment
    name: rust-api
  minReplicas: 1
  maxReplicas: 3
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 30
```

- **Scale up:** estabilización 30s, máx 1 pod cada 30s
- **Scale down:** estabilización 120s, máx 1 pod cada 60s
- **Requisito:** metrics-server activo en GKE (habilitado por defecto)

Verificar estado del HPA:
```bash
kubectl get hpa rust-api-hpa -n mumk8s -w
```

---

## 9. Publicación y Consumo de Imágenes desde Zot

### Instalación de Zot (VM N1 externa)

```bash
# Copiar scripts a la VM
gcloud compute scp zot/install-zot.sh zot/config.json zot-registry:~/

# Ejecutar instalación
gcloud compute ssh zot-registry -- sudo bash install-zot.sh
```

### Build y push de imágenes

```bash
export ZOT_HOST=<IP-ZOT>:5000

# Build y push de todas las imágenes
bash infra/build-push.sh

# Push individual (ejemplo)
docker build -t ${ZOT_HOST}/mumk8s/rust-api:latest ./rust-api
docker push ${ZOT_HOST}/mumk8s/rust-api:latest
```

### Pull desde GKE (nodos)

Los nodos GKE deben poder acceder al registry HTTP (inseguro). Esto se configura mediante un `ContainerRuntimeConfig` o directamente en la configuración del cluster node pool si es necesario. Para desarrollo, se puede configurar Docker con `--insecure-registry`.

---

## 10. OCI Artifact

### Archivo: `war_report.proto`

El archivo `go-grpc-server/proto/war_report.proto` define el contrato gRPC del sistema. Se publica como OCI Artifact en Zot para:
1. Versionado centralizado del schema gRPC
2. Distribución a equipos que necesiten implementar clientes
3. Trazabilidad del contrato en el mismo registry que las imágenes

**Herramienta:** [`oras`](https://oras.land) (OCI Registry As Storage)

### Subir el artifact

```bash
export ZOT_HOST=<IP-ZOT>:5000

# Subir proto
bash zot/oci-artifact.sh push

# Subir dashboard de Grafana también como artifact
bash zot/oci-artifact.sh push-dash

# Subir ambos de una vez
bash zot/oci-artifact.sh all
```

### Descargar el artifact

```bash
# Descargar proto (p.ej. en un nuevo microservicio cliente)
bash zot/oci-artifact.sh pull ./mi-proyecto/proto

# Descargar dashboard (en la VM de Grafana para importarlo)
bash zot/oci-artifact.sh pull-dash /tmp
```

### Verificar en el registry

```bash
# Listar artifacts disponibles
curl http://${ZOT_HOST}/v2/_catalog

# Ver tags del proto
curl http://${ZOT_HOST}/v2/mumk8s/grpc-proto/tags/list
```

---

## 11. Estructura de Datos en Valkey

### Por país (`{PAIS}` = USA | RUS | CHN | ESP | GTM)

| Clave | Tipo Redis | Descripción |
|---|---|---|
| `{PAIS}:count` | String | Total de reportes del país |
| `{PAIS}:warplanes:total` | String | Suma acumulada de aviones |
| `{PAIS}:warships:total` | String | Suma acumulada de barcos |
| `{PAIS}:warplanes:max` | String | Máximo de aviones registrado |
| `{PAIS}:warplanes:min` | String | Mínimo de aviones registrado |
| `{PAIS}:warships:max` | String | Máximo de barcos registrado |
| `{PAIS}:warships:min` | String | Mínimo de barcos registrado |
| `{PAIS}:warplanes:freq` | Hash | `{valor} → frecuencia` (para moda) |
| `{PAIS}:warships:freq` | Hash | `{valor} → frecuencia` (para moda) |
| `{PAIS}:warplanes:timeseries` | Sorted Set | `score=unix_ms`, `member="{valor}:{seq}"` |
| `{PAIS}:warships:timeseries` | Sorted Set | `score=unix_ms`, `member="{valor}:{seq}"` |
| `{PAIS}:warplanes:ts:seq` | String | Contador de secuencia para time series |
| `{PAIS}:warships:ts:seq` | String | Contador de secuencia para time series |

### Global

| Clave | Tipo | Descripción |
|---|---|---|
| `global:count` | String | Total de reportes de todos los países |
| `global:warplanes:max` | String | Máximo global de aviones |
| `global:warplanes:min` | String | Mínimo global de aviones |
| `global:warships:max` | String | Máximo global de barcos |
| `global:warships:min` | String | Mínimo global de barcos |
| `global:warplanes:freq` | Hash | Frecuencias globales de aviones (para moda) |
| `global:warships:freq` | Hash | Frecuencias globales de barcos (para moda) |
| `global:warplanes:ranking` | Sorted Set | `score=total_acumulado`, `member=PAIS` |
| `global:warships:ranking` | Sorted Set | `score=total_acumulado`, `member=PAIS` |

---

## 12. Dashboard de Grafana

El dashboard `k8s/grafana/dashboard.json` contiene los siguientes paneles:

| # | Panel | Tipo | Fuente |
|---|---|---|---|
| 1 | País Asignado (RUS) | Text | Estático |
| 2 | Total Reportes Recibidos | Stat | `GET global:count` |
| 3 | Máx. Aviones Global | Stat | `GET global:warplanes:max` |
| 4 | Mín. Aviones Global | Stat | `GET global:warplanes:min` |
| 5 | Máx. Barcos Global | Stat | `GET global:warships:max` |
| 6 | Mín. Barcos Global | Stat | `GET global:warships:min` |
| 7 | Moda Aviones Global | Stat | `EVAL lua_script global:warplanes:freq` |
| 8 | Moda Barcos Global | Stat | `EVAL lua_script global:warships:freq` |
| 9 | Top Países — Aviones | Bar Chart | `ZREVRANGE global:warplanes:ranking 0 4` |
| 10 | Top Países — Barcos | Bar Chart | `ZREVRANGE global:warships:ranking 0 4` |
| 11 | RUS — Aviones (Serie Temporal) | Time Series | `ZRANGEBYSCORE RUS:warplanes:timeseries` |
| 12 | RUS — Barcos (Serie Temporal) | Time Series | `ZRANGEBYSCORE RUS:warships:timeseries` |

**Nota sobre series temporales:** Los members del sorted set tienen formato `{valor}:{seq}`. El panel usa la transformación `Extract fields` de Grafana con delimitador `:` para aislar el valor numérico. El score es el Unix timestamp en ms usado como eje de tiempo.

**Moda (Lua script):** Se usa `EVAL` con un script Lua que recorre el hash de frecuencias y retorna la clave con mayor conteo:
```lua
local h = redis.call('HGETALL', KEYS[1])
local mv = '0'
local mc = 0
for i = 1, #h, 2 do
  local c = tonumber(h[i+1])
  if c > mc then mc = c; mv = h[i] end
end
return mv
```

---

## 13. Pruebas Comparativas: 1 vs 2 Réplicas (Go Writers)

La rúbrica requiere documentar el comportamiento del sistema con **1 réplica** y **2 réplicas** en los Go Writers (`go-grpc-server` y `go-rabbitmq-writer`) bajo carga generada por Locust.

### Configuración de la prueba

| Parámetro Locust | Valor |
|---|---|
| Usuarios concurrentes | 100 |
| Spawn rate | 10 usuarios/s |
| Duración | 3 minutos por escenario |
| Endpoint | `POST /grpc-201114493` |

### Comandos para cambiar réplicas

```bash
# --- Escenario A: 1 réplica (baseline) ---
kubectl scale deployment go-grpc-server      --replicas=1 -n mumk8s
kubectl scale deployment go-rabbitmq-writer  --replicas=1 -n mumk8s
kubectl get pods -n mumk8s -l 'app in (go-grpc-server,go-rabbitmq-writer)'

# Ejecutar Locust 3 minutos, anotar métricas, luego:

# --- Escenario B: 2 réplicas ---
kubectl scale deployment go-grpc-server      --replicas=2 -n mumk8s
kubectl scale deployment go-rabbitmq-writer  --replicas=2 -n mumk8s
kubectl get pods -n mumk8s -l 'app in (go-grpc-server,go-rabbitmq-writer)'

# Ejecutar Locust otros 3 minutos con la misma configuración
```

### Métricas a recolectar durante cada escenario

```bash
# Throughput de la cola RabbitMQ (mensajes/s)
kubectl exec -n mumk8s deploy/rabbitmq -- \
  rabbitmqctl list_queues name messages_ready messages_unacknowledged

# CPU y memoria de los pods
kubectl top pods -n mumk8s

# Estado del HPA en rust-api
kubectl get hpa rust-api-hpa -n mumk8s

# Total de reportes almacenados en Valkey
kubectl run redis-check --image=redis:7-alpine --rm -it -n mumk8s \
  --restart=Never -- redis-cli -h valkey-vm-svc GET global:count
```

### Tabla de resultados esperada

Completar durante la ejecución real en GCP:

| Métrica | 1 Réplica | 2 Réplicas | Mejora |
|---|---|---|---|
| Requests/s (Locust) | ___ | ___ | ___% |
| Latencia promedio (ms) | ___ | ___ | ___% |
| Latencia p95 (ms) | ___ | ___ | ___% |
| Tasa de error (%) | ___ | ___ | ___ |
| Mensajes en cola (pico) | ___ | ___ | ___ |
| CPU go-grpc-server (%) | ___ | ___ | ___ |
| CPU go-rabbitmq-writer (%) | ___ | ___ | ___ |
| Total reportes en Valkey | ___ | ___ | ___ |

### Análisis esperado

Con **1 réplica**, los Go Writers procesan las peticiones de forma secuencial bajo el mismo pod. Al aumentar la carga el CPU sube, la latencia aumenta y pueden acumularse mensajes en la cola RabbitMQ.

Con **2 réplicas**, Kubernetes distribuye las peticiones entre ambos pods via ClusterIP (round-robin). Se espera:
- Mayor throughput (reducción de latencia p95)
- CPU por pod aprox. 50% del escenario anterior
- Cola RabbitMQ con menor profundidad acumulada
- La cola `war_reports` sigue siendo consumida por la misma instancia de `go-consumer`

### Nota sobre HPA en rust-api

Durante ambos escenarios, monitorear si el HPA activa el scale-up de `rust-api`:

```bash
kubectl get hpa rust-api-hpa -n mumk8s -w
```

Si CPU supera el 30%, `rust-api` escala de 1 a máximo 3 réplicas automáticamente. Documentar en qué momento ocurre el scale-up y scale-down.

---

## 14. Pruebas de Funcionalidad

### Prueba de flujo completo (smoke test)

```bash
GATEWAY_IP=<ip-gateway>

curl -X POST "http://${GATEWAY_IP}/grpc-201114493" \
  -H "Content-Type: application/json" \
  -d '{"country":"RUS","warplanes_in_air":25,"warships_in_water":10,"timestamp":"2026-04-01T12:00:00Z"}'

# Respuesta esperada:
# {"status":"ok","country":"RUS"}
```

### Prueba de escala con Locust

1. Arrancar Locust: `docker run -p 8089:8089 ${ZOT_HOST}/mumk8s/locust:latest --host http://${GATEWAY_IP}`
2. Configurar 50 usuarios con spawn rate 5/s
3. Verificar HPA escalando: `kubectl get hpa rust-api-hpa -n mumk8s -w`
4. Observar en Grafana el incremento de reportes por país

### Prueba de resiliencia

```bash
# Simular falla de go-rabbitmq-writer
kubectl delete pod -n mumk8s -l app=go-rabbitmq-writer

# El go-grpc-server retornará error al cliente durante la reconexión
# Kubernetes reinicia el pod automáticamente (restartPolicy: Always)
# go-rabbitmq-writer reconecta a RabbitMQ al arrancar
```

### Verificar datos en Valkey

```bash
# Conectarse al pod de redis-cli (temporal)
kubectl run redis-cli --image=redis:7-alpine --rm -it -n mumk8s -- \
  redis-cli -h valkey-vm-svc -p 6379

# Comandos de verificación:
GET global:count
GET RUS:count
GET global:warplanes:max
ZREVRANGE global:warplanes:ranking 0 4 WITHSCORES
HGETALL global:warplanes:freq
```

---

## 15. Conclusiones

1. **Separación de responsabilidades:** Dividir el flujo en 4 deployments Go (client, gRPC server, writer, consumer) permite escalar y desplegar cada componente de forma independiente.

2. **KubeVirt:** Permite ejecutar VMs (Valkey y Grafana) dentro del clúster Kubernetes, manteniendo la arquitectura unificada y gestionada por las mismas herramientas (kubectl, namespaces).

3. **Gateway API vs Ingress:** Gateway API ofrece mayor flexibilidad y separación de roles. El `GatewayClass` gestiona la infraestructura; el `HTTPRoute` gestiona el enrutamiento de la aplicación.

4. **HPA:** Con CPU target de 30%, `rust-api` escala rápidamente ante picos de carga de Locust, demostrando la elasticidad de la arquitectura.

5. **OCI Artifacts:** Usar Zot tanto para imágenes Docker como para artifacts (.proto, dashboard JSON) centraliza todos los artefactos del sistema en un único registry bajo control del equipo.

6. **Valkey + Sorted Sets para series temporales:** Usar sorted sets con score=Unix_ms y members únicos (valor:seq) permite una implementación de time series sin módulos adicionales, compatible con el plugin `redis-datasource` de Grafana mediante transformaciones de campos.
