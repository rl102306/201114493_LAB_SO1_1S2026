# M.U.M.N.K8s — Monitoreo de Unidades Militares en la Nube con Kubernetes
**Carnet:** 201114493 | **País asignado:** RUS

## Arquitectura

```
Locust
  │ POST /grpc-201114493
  ▼
Gateway API (GKE L7)
  │
  ▼
[rust-api]  ─── HPA (1-3 pods, CPU > 30%)
  │ HTTP /report
  ▼
[go-grpc-client]
  │ gRPC SendReport
  ▼
[go-grpc-server]
  │ AMQP publish
  ▼
RabbitMQ (queue: war_reports)
  │ AMQP consume
  ▼
[go-consumer]
  │ SET/INCR/HINCRBY
  ▼
Valkey (KubeVirt VM1)
  ▲
Grafana (KubeVirt VM2)
```

## Estructura de directorios

```
proyecto3/
├── rust-api/           API REST Axum (Rust)
├── go-grpc-client/     Cliente gRPC (Go)
├── go-grpc-server/     Servidor gRPC + RabbitMQ writer (Go)
├── go-consumer/        Consumidor RabbitMQ + Valkey writer (Go)
├── locust/             Generador de carga (Python)
└── k8s/
    ├── namespace.yaml
    ├── configmaps/
    ├── gateway/        GatewayClass, Gateway, HTTPRoute
    ├── deployments/    rust-api, go-grpc-*, go-consumer, rabbitmq
    ├── services/
    ├── hpa/            HPA para rust-api
    └── kubevirt/       VMs para Valkey y Grafana
```

## Prerrequisitos

- GKE cluster con Gateway API habilitado
- KubeVirt instalado en el cluster
- Registro Zot corriendo en VM externa (`ZOT_HOST`)
- `kubectl`, `docker`, `go 1.22+`, `rust 1.78+`

## Variables de entorno clave

| Variable | Descripción | Default |
|---|---|---|
| `ZOT_HOST` | Host del registry Zot | — |
| `GRPC_CLIENT_URL` | URL del go-grpc-client | `http://go-grpc-client-svc:8080` |
| `GRPC_SERVER_ADDR` | Dirección gRPC server | `go-grpc-server-svc:50051` |
| `RABBITMQ_URL` | URL AMQP de RabbitMQ | `amqp://guest:guest@rabbitmq-svc:5672/` |
| `RABBITMQ_QUEUE` | Nombre de la cola | `war_reports` |
| `VALKEY_ADDR` | Dirección Valkey | `valkey-vm-svc:6379` |

## Build y push de imágenes

```bash
export ZOT_HOST=<ip-o-hostname-del-registry>

# rust-api
docker build -t ${ZOT_HOST}/mumk8s/rust-api:latest ./rust-api
docker push ${ZOT_HOST}/mumk8s/rust-api:latest

# go-grpc-client
docker build -t ${ZOT_HOST}/mumk8s/go-grpc-client:latest ./go-grpc-client
docker push ${ZOT_HOST}/mumk8s/go-grpc-client:latest

# go-grpc-server
docker build -t ${ZOT_HOST}/mumk8s/go-grpc-server:latest ./go-grpc-server
docker push ${ZOT_HOST}/mumk8s/go-grpc-server:latest

# go-consumer
docker build -t ${ZOT_HOST}/mumk8s/go-consumer:latest ./go-consumer
docker push ${ZOT_HOST}/mumk8s/go-consumer:latest

# locust
docker build -t ${ZOT_HOST}/mumk8s/locust:latest ./locust
docker push ${ZOT_HOST}/mumk8s/locust:latest
```

## Despliegue en GKE

```bash
# 1. Actualizar imágenes en deployments (sustituir variable)
sed -i "s|\${ZOT_HOST}|${ZOT_HOST}|g" k8s/deployments/*.yaml

# 2. Aplicar en orden
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmaps/
kubectl apply -f k8s/kubevirt/        # VMs Valkey y Grafana
kubectl apply -f k8s/deployments/rabbitmq.yaml
kubectl apply -f k8s/services/rabbitmq-svc.yaml
# Esperar RabbitMQ ready
kubectl wait --for=condition=available deployment/rabbitmq -n mumk8s --timeout=120s

kubectl apply -f k8s/deployments/go-grpc-server.yaml
kubectl apply -f k8s/services/go-grpc-server-svc.yaml
kubectl apply -f k8s/deployments/go-grpc-client.yaml
kubectl apply -f k8s/services/go-grpc-client-svc.yaml
kubectl apply -f k8s/deployments/go-consumer.yaml
kubectl apply -f k8s/deployments/rust-api.yaml
kubectl apply -f k8s/services/rust-api-svc.yaml
kubectl apply -f k8s/hpa/rust-api-hpa.yaml

# 3. Gateway API
kubectl apply -f k8s/gateway/gateway-class.yaml
kubectl apply -f k8s/gateway/gateway.yaml
kubectl apply -f k8s/gateway/http-route.yaml

# 4. Obtener IP del Gateway
kubectl get gateway mumk8s-gateway -n mumk8s
```

## Ejecutar Locust

```bash
GATEWAY_IP=$(kubectl get gateway mumk8s-gateway -n mumk8s \
  -o jsonpath='{.status.addresses[0].value}')

docker run -p 8089:8089 \
  ${ZOT_HOST}/mumk8s/locust:latest \
  --host http://${GATEWAY_IP}
# Abrir http://localhost:8089
```

## Estructura de datos en Valkey

Por cada país (`USA`, `RUS`, `CHN`, `ESP`, `GTM`):

| Clave | Tipo | Descripción |
|---|---|---|
| `{país}:count` | String | Total de reportes |
| `{país}:warplanes:total` | String | Suma de aviones |
| `{país}:warships:total` | String | Suma de barcos |
| `{país}:warplanes:max` | String | Máximo de aviones |
| `{país}:warplanes:min` | String | Mínimo de aviones |
| `{país}:warships:max` | String | Máximo de barcos |
| `{país}:warships:min` | String | Mínimo de barcos |
| `{país}:warplanes:freq` | Hash | value→count (para moda) |
| `{país}:warships:freq` | Hash | value→count (para moda) |
| `global:count` | String | Total global de reportes |

## Grafana — Queries de ejemplo (plugin Redis)

```
# Total reportes por país
MGET RUS:count USA:count CHN:count ESP:count GTM:count

# Máximo aviones RUS
GET RUS:warplanes:max

# Moda aviones RUS (top 1 del hash de frecuencias)
HGETALL RUS:warplanes:freq
```

## Mensaje JSON esperado

```json
{
  "country": "RUS",
  "warplanes_in_air": 42,
  "warships_in_water": 14,
  "timestamp": "2026-03-12T20:15:30Z"
}
```

- `country`: `USA | RUS | CHN | ESP | GTM`
- `warplanes_in_air`: 0–50
- `warships_in_water`: 0–30
