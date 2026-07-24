# Comandos de Calificación - Proyecto M.U.M.N.K8s
## Carnet: 201114493

---

## 1. Infraestructura Base y Zot (Fuera del Clúster)

### Verificar acceso a VM de Zot
```bash
gcloud compute ssh zot-registry --zone=us-central1-a
```

### Verificar proceso de Zot
```bash
systemctl status zot
```

### Verificar imágenes subidas
```bash
curl http://34.172.8.2:5000/v2/_catalog
```

---

## 2. Orquestación y Red (GKE & Gateway API)

### Verificar Gateway
```bash
kubectl get gateway -n mumk8s
```

### Verificar Rutas
```bash
kubectl get httproute -n mumk8s
```

### Probar endpoint
```bash
curl -X POST http://34.160.149.45/grpc-201114493 \
  -H "Content-Type: application/json" \
  -d '{"country":"GTM","warplanes_in_air":10,"warships_in_water":5,"timestamp":"2026-05-02T00:00:00Z"}'
```

---

## 3. Lógica de Aplicación (Rust & Go)

### Verificar HPA
```bash
kubectl get hpa -n mumk8s
```

### Logs rust-api
```bash
kubectl logs deployment/rust-api -n mumk8s --tail=20
```

### Logs go-grpc-server
```bash
kubectl logs deployment/go-grpc-server -n mumk8s --tail=20
```

---

## 4. KubeVirt (Valkey & Grafana)

### Listar VirtualMachines
```bash
kubectl get vms -n mumk8s
```

### Verificar estado VMI
```bash
kubectl get vmi -n mumk8s
```

### Describir VMI
```bash
kubectl describe vmi valkey-vm -n mumk8s
kubectl describe vmi grafana-vm -n mumk8s
```

### Acceso consola Valkey VM
```bash
virtctl console valkey-vm -n mumk8s
```
> Usuario: `ubuntu` | Contraseña: `ubuntu123`

### Persistencia Valkey (dentro del VM)
```bash
redis-cli ping
redis-cli dbsize
redis-cli info persistence
```

### Acceso consola Grafana VM
```bash
virtctl console grafana-vm -n mumk8s
```
> Usuario: `ubuntu` | Contraseña: `ubuntu123`

---

## 5. Mensajería (RabbitMQ)

### Verificar colas
```bash
kubectl exec -it -n mumk8s deployment/rabbitmq -- rabbitmqctl list_queues
```

### Logs go-consumer
```bash
kubectl logs deployment/go-consumer -n mumk8s --tail=20
```

---

## 6. Pruebas de Carga (Locust)

### Ejecutar Locust
```bash
cd /home/ronaldo-lara/201114493_LAB_SO1_1S2026/proyecto3/locust
python3 -m locust --host=http://34.160.149.45 --users=50 --spawn-rate=5 --headless --run-time=5m
```

### Verificar HPA escalando (en otra terminal)
```bash
kubectl get hpa -n mumk8s -w
```

---

## 7. Grafana

### Acceder al dashboard
```
http://34.31.68.145:3000
```
> Usuario: `admin` | Contraseña: `admin`

### Verificar servicio
```bash
curl -s http://34.31.68.145:3000/api/health
```

---

## Estado General del Cluster

```bash
kubectl get pods -n mumk8s
kubectl get vmi -n mumk8s
kubectl get hpa -n mumk8s
kubectl get gateway -n mumk8s
kubectl get httproute -n mumk8s
```
