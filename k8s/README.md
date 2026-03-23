# Kubernetes Deployment — Step-by-Step Log

All commands run from the **repo root** unless stated otherwise.
Namespace for everything: `obs-pipeline`.

---

## Prerequisites

Enable Kubernetes in Docker Desktop → Settings → Kubernetes → Enable → Apply & Restart.

```bash
# Verify tools
kubectl version --client          # should show v1.34+
kubectl get nodes                 # should show "docker-desktop   Ready"
helm version                      # install via: scoop install helm

# Add Helm repos (needed even though we ended up not using bitnami — kept for reference)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

> **Note on Bitnami:** Bitnami container images are paywalled since August 2025.
> All infra (Kafka, TimescaleDB, Redis) is deployed using raw YAML with the same
> images as `docker-compose.yml` — no Helm charts required.

---

## Step 1 — Namespace

```bash
kubectl apply -f k8s/namespace.yaml
kubectl get namespace obs-pipeline
```

**What is a Namespace?**
A virtual cluster inside your k8s cluster. All resources live in `obs-pipeline`.
Every `kubectl` command needs `-n obs-pipeline`, otherwise you're talking to `default`.

---

## Step 2 — Kafka StatefulSet

```bash
kubectl apply -f k8s/infra/kafka/
kubectl rollout status statefulset/obs-kafka -n obs-pipeline --timeout=180s
kubectl get pods -n obs-pipeline
```

**Files:**
- `headless-service.yaml` — gives each pod a stable DNS name (`obs-kafka-0.obs-kafka-headless`)
- `service.yaml` — ClusterIP service; app services connect to `obs-kafka:9092`
- `statefulset.yaml` — the Kafka pod with a 1Gi PVC for commit logs

**Key concepts learned:**

| Concept | Detail |
|---------|--------|
| StatefulSet vs Deployment | StatefulSet = stable pod name + dedicated PVC per pod |
| Headless service | `clusterIP: None` — DNS per pod instead of load-balanced VIP |
| ClusterIP service | Load-balanced stable address; only routes to Ready pods |
| PVC via volumeClaimTemplate | Each pod gets its own disk; survives pod restarts |
| tcpSocket probe | Avoids ClusterIP deadlock (exec probe would deadlock on startup) |

**Bugs hit and fixed:**

1. **ClusterIP deadlock on quorum voters** — configured `KAFKA_CONTROLLER_QUORUM_VOTERS=1@obs-kafka:29093`. The ClusterIP has no endpoints until the pod is Ready, so the pod can't reach its own controller to become Ready — infinite loop. Fix: use `localhost:29093` (broker and controller are the same JVM).

2. **Probe timeout** — default `timeoutSeconds: 1s` too short for `kafka-topics` command. Fix: `timeoutSeconds: 10`.

3. **exec probe deadlock** — `kafka-topics --bootstrap-server localhost:9092` connects, gets told "broker is at `obs-kafka:9092`" in the metadata response, then reconnects to the ClusterIP — which again has no endpoints. Fix: use `tcpSocket` probe instead (just checks if the port is open, no client protocol).

---

## Step 3 — Kafka Init Job

```bash
kubectl apply -f k8s/infra/kafka-init-job.yaml
kubectl wait --for=condition=complete job/kafka-init -n obs-pipeline --timeout=60s
kubectl logs job/kafka-init -n obs-pipeline
```

**What it does:** Creates the 4 Kafka topics: `events.raw`, `events.enriched`, `events.dlq`, `events.alerts`.

**Key concepts learned:**

| Concept | Detail |
|---------|--------|
| Job | Runs a pod once to completion — k8s equivalent of `restart: "no"` in docker-compose |
| ConfigMap as file | ConfigMaps can hold text files, not just env vars. Mounted into the pod as a script. |
| ttlSecondsAfterFinished | Auto-deletes the completed Job pod after N seconds (keeps `kubectl get pods` clean) |
| restartPolicy: OnFailure | Retry on non-zero exit, stop on success |

> **Why a Job instead of letting each service create its own topics?**
> Multiple services starting simultaneously could race to create the same topic with
> different configs. One authoritative Job avoids this.

---

## Step 4 — TimescaleDB StatefulSet

```bash
kubectl apply -f k8s/infra/timescaledb/
kubectl rollout status statefulset/timescaledb -n obs-pipeline --timeout=120s
kubectl get pods -n obs-pipeline
```

**Files:**
- `service.yaml` — ClusterIP; app services connect to `timescaledb:5432`
- `statefulset.yaml` — the TimescaleDB pod with a 2Gi PVC

**Why no headless service?**
TimescaleDB is a single-node database. No peer discovery needed — clients just connect
to the ClusterIP service. Contrast with Kafka where brokers need per-pod DNS to find each other.

**Key concepts:**

| Concept | Detail |
|---------|--------|
| exec probe (pg_isready) | Built-in Postgres health check tool. No ClusterIP deadlock because it connects to `localhost` directly, not via the service. |
| PGDATA env var | Postgres requires the data dir to be a subdirectory inside the mount point |

---

## Step 5 — DB Init Job

```bash
kubectl apply -f k8s/infra/db-init-job.yaml
kubectl wait --for=condition=complete job/db-init -n obs-pipeline --timeout=120s
kubectl logs job/db-init -n obs-pipeline
```

**What it does:** Creates the `events` hypertable, indexes, continuous aggregate (`events_per_minute`), and retention policy. SQL extracted from `services/enricher/internal/sink/timescale.go`.

**Key concepts:**

| Concept | Detail |
|---------|--------|
| Init container | Runs before the main container. Here: waits until `timescaledb:5432` is open (`nc -z`). |
| ConfigMap as SQL file | Same pattern as the Kafka init script — SQL stored in ConfigMap, mounted into pod |
| ON_ERROR_STOP=1 | psql flag: abort on first SQL error instead of continuing |

---

## Step 6 — Redis Deployment

```bash
kubectl apply -f k8s/infra/redis/
kubectl rollout status deployment/redis -n obs-pipeline --timeout=60s
```

**Files:**
- `deployment.yaml` — Redis pod (ephemeral counters, no PVC needed)
- `service.yaml` — ClusterIP `redis:6379`

**Why Deployment and NOT StatefulSet?**

| | Kafka | TimescaleDB | Redis |
|-|-------|-------------|-------|
| Kind | StatefulSet | StatefulSet | Deployment |
| Pod name | `obs-kafka-0` (stable) | `timescaledb-0` (stable) | `redis-xxxxx-xxxxx` (random) |
| Why | KRaft quorum voters need stable name | Data must survive restarts | Counters are ephemeral |

The random suffix in the pod name is the clearest visual signal: Deployment = interchangeable pods.

---

## Step 7 — ConfigMap and Secret

```bash
kubectl apply -f k8s/manifests/configmap.yaml
kubectl apply -f k8s/manifests/secret.yaml
kubectl describe configmap obs-config -n obs-pipeline
kubectl get secret obs-secrets -n obs-pipeline
```

**Key concepts:**

| | ConfigMap | Secret |
|-|-----------|--------|
| Contents | Non-sensitive: Kafka address, topic names, file paths | Sensitive: DATABASE_URL with password |
| In pod | `envFrom: configMapRef` (all keys at once) | `secretKeyRef` (individual keys) |
| Storage | Plain text | Base64-encoded at rest |

`stringData` in the Secret lets you write plain text — k8s encodes it on `kubectl apply`.
Without `stringData` you'd need to manually base64-encode every value.

---

## Step 8 — Update Enricher Dockerfile

Added two lines to `services/enricher/Dockerfile`:

```dockerfile
COPY libs/event-schema/service-registry.json ./service-registry.json
COPY infra/geoip/GeoLite2-City.mmdb ./GeoLite2-City.mmdb
```

**Why bake files into the image vs mounting them?**

| Option | Works for | Not for |
|--------|-----------|---------|
| `COPY` into image | Any file ✓ | Secrets, frequently changing files |
| ConfigMap | Small text files (<1MB) | Binary files like `.mmdb` (60MB) |
| PersistentVolume | Large/changing data | One-off static files |

In docker-compose these were bind-mounted from the laptop. In k8s, the pod runs on
a cluster node — your laptop's filesystem isn't there.

---

## Step 9 — Build All Docker Images

```bash
# Run from repo root
docker build -t obs-ingestor:latest -f services/ingestor/Dockerfile .
docker build -t obs-enricher:latest -f services/enricher/Dockerfile .
docker build -t obs-detector:latest services/detector/
docker build -t obs-query-api:latest services/query-api/
docker build -t obs-dashboard:latest services/dashboard/

# Verify
docker images | grep obs-
```

**Key concept — imagePullPolicy: IfNotPresent:**
Docker Desktop's k8s shares the same image store as the Docker daemon. `docker build`
here = image available to k8s immediately. Without `imagePullPolicy: IfNotPresent`,
k8s would fail with `ImagePullBackOff` trying to pull `obs-ingestor` from Docker Hub.

**Multi-stage builds:** The final image only contains the runtime binary — not the Go
compiler, npm, or Python build tools.

| Image | Final size | Base |
|-------|-----------|------|
| obs-dashboard | 26MB | nginx:alpine |
| obs-ingestor | 54MB | debian:bookworm-slim |
| obs-detector | 57MB | python:3.12-slim |
| obs-query-api | 76MB | node:24-alpine |
| obs-enricher | 78MB | debian:bookworm-slim (+ 60MB GeoIP) |

---

## Steps 10–14 — App Service Deployments

```bash
kubectl apply -f k8s/manifests/ingestor/
kubectl apply -f k8s/manifests/enricher/
kubectl apply -f k8s/manifests/detector/
kubectl apply -f k8s/manifests/query-api/
kubectl apply -f k8s/manifests/dashboard/

# Wait for all to be ready
kubectl rollout status deployment/ingestor deployment/enricher deployment/detector deployment/query-api deployment/dashboard -n obs-pipeline --timeout=180s
```

**Init container chains (run sequentially before the main container):**

| Service | Init containers | Why |
|---------|----------------|-----|
| Ingestor | kafka | Can't produce events without Kafka |
| Enricher | kafka → schema | Needs Kafka + DB table to exist |
| Detector | kafka → schema → redis | Needs all three |
| Query-API | kafka → schema → redis | Needs all three |
| Dashboard | *(none)* | Static nginx, no runtime dependencies |

**`wait-for-schema` checks the table, not just the port:**
```bash
until psql "$DATABASE_URL" -c "\dt events" | grep -q events; do sleep 3; done
```
Port 5432 open ≠ `db-init-job` finished. This waits until the `events` table actually exists.

**Probe types:**

| Service | Probe type | Why |
|---------|-----------|-----|
| Ingestor, Query-API | `httpGet /healthz` | Has HTTP server |
| Enricher, Detector | `tcpSocket` on metrics port | No HTTP endpoint |
| Dashboard | `httpGet /` | nginx static files |

**liveness vs readiness:**

| Probe | Failing effect |
|-------|---------------|
| `readinessProbe` | Pod removed from Service endpoints — no traffic sent |
| `livenessProbe` | Pod killed and restarted |

---

## Step 15 — Ingress Controller + Ingress Rules

```bash
# Install nginx Ingress Controller (once per cluster)
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.10.0/deploy/static/provider/cloud/deploy.yaml
kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout=120s

# Apply routing rules
kubectl apply -f k8s/ingress.yaml
kubectl get ingress -n obs-pipeline  # ADDRESS should show "localhost"

# Smoke tests
curl http://localhost/api/healthz    # {"status":"ok"}
curl -s -o /dev/null -w "%{http_code}" http://localhost/  # 200
```

**Key concepts:**

| Thing | What it is |
|-------|-----------|
| Ingress Controller | A running nginx pod that watches for Ingress resources |
| Ingress resource | A routing rule — "send `/api/*` to service X, `/` to service Y" |
| ingressClassName | Picks which controller handles this Ingress |

**Path rewrite:** The ingestor has no `/api` prefix on its routes. The rewrite annotation strips it:
```
browser: GET /api/v1/events
ingress nginx rewrites → GET /v1/events → ingestor:8080
```

**Traffic flow:**
```
localhost:80 → ingress-nginx pod → obs-pipeline/ingestor:8080  (for /api/*)
                                 → obs-pipeline/dashboard:3000  (for /*)
                                       ↓ nginx.conf proxy_pass
                                 → obs-pipeline/query-api:4000  (for /graphql)
```

---

## Useful Commands

```bash
# See everything in the namespace
kubectl get all -n obs-pipeline

# Watch pods in real time
kubectl get pods -n obs-pipeline -w

# Stream logs from a deployment
kubectl logs -n obs-pipeline deployment/<name> -f

# See why a pod isn't starting
kubectl describe pod <pod-name> -n obs-pipeline

# Check init container logs (when pod is stuck in Init state)
kubectl logs <pod-name> -n obs-pipeline -c wait-for-kafka

# Exec into a running pod
kubectl exec -it <pod-name> -n obs-pipeline -- bash

# Delete everything and start over
kubectl delete namespace obs-pipeline

# Makefile shortcuts (run from k8s/ directory)
make status                 # kubectl get all -n obs-pipeline
make logs SVC=ingestor      # stream ingestor logs
make deploy-all             # deploy infra + app in one shot
make teardown               # delete the namespace
```

---

## Current State

```
obs-kafka-0                  1/1  Running    Kafka broker + controller (KRaft)
timescaledb-0                1/1  Running    TimescaleDB (events hypertable + aggregate)
redis-xxxxx                  1/1  Running    Redis (ephemeral state)
ingestor-xxxxx               1/1  Running    Go/Gin HTTP → events.raw
enricher-xxxxx               1/1  Running    Go enricher → TimescaleDB + events.enriched
detector-xxxxx               1/1  Running    Python anomaly detector → events.alerts
query-api-xxxxx              1/1  Running    Node GraphQL API
dashboard-xxxxx              1/1  Running    React dashboard (nginx)
kafka-init                   Completed       Created 4 Kafka topics
db-init                      Completed       Schema migrations
ingress-nginx-controller     1/1  Running    Routes localhost → cluster services
```

Run `kubectl get pods -n obs-pipeline` to see live status.
Open `http://localhost` in a browser to see the dashboard.
