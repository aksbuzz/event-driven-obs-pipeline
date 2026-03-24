# Helm Charts — Observability Pipeline

This directory contains two Helm charts that replace the raw `k8s/` YAML files with a
parameterised, upgradable deployment. Every tuneable (image tag, storage size, replica
count, port number) is a value — the templates themselves never change between environments.

| Chart | Manages | Deploy order |
|---|---|---|
| `obs-infra` | Kafka, TimescaleDB, Redis | 1st — infra must exist before apps start |
| `obs-pipeline` | 5 app services + RBAC + NetworkPolicy + HPA | 2nd — depends on infra |

---

## What is Helm?

Helm is the package manager for Kubernetes. Just as `npm` bundles a Node application's
dependencies into a `package.json` and lets you install or upgrade the whole thing with
one command, Helm bundles a set of Kubernetes manifests into a **chart** and lets you
install, upgrade, roll back, and uninstall that whole set as a single unit.

Without Helm you manage a directory of raw YAML files. Every environment difference
(dev vs staging vs prod) means either duplicating files or using `sed` to patch values
in CI. With Helm, one chart + one `values.yaml` override file is all you need per
environment. The templates are reusable; only the values differ.

Helm also tracks **releases** — every `helm install` or `helm upgrade` is recorded in
the cluster (as a Secret in the target namespace). This means you can do
`helm rollback obs-pipeline 2` to instantly revert to the previous revision without
touching your YAML files.

---

## Prerequisites

### 1. Helm itself

```bash
# macOS / Linux (Homebrew)
brew install helm

# Windows (Scoop)
scoop install helm

# Or download the binary: https://github.com/helm/helm/releases
```

Verify:

```bash
helm version
# version.BuildInfo{Version:"v3.x.x", ...}
```

You need **Helm 3**. Helm 2 required a server-side component called Tiller; Helm 3 is
client-only and talks directly to the Kubernetes API.

### 2. A running Kubernetes cluster

For local development, **Docker Desktop** ships a single-node cluster you can enable in
Settings > Kubernetes > Enable Kubernetes. Alternatively use **minikube** or **kind**.

```bash
kubectl cluster-info   # should show a running control plane
```

### 3. metrics-server (required for HPA)

The `obs-pipeline` chart includes a HorizontalPodAutoscaler for the ingestor. HPA reads
CPU/memory metrics from `metrics-server`. Docker Desktop does not ship it by default.

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# On Docker Desktop you also need to patch the deployment to skip TLS verification:
kubectl patch deployment metrics-server -n kube-system \
  --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

### 4. nginx Ingress controller (required for external access)

The `obs-pipeline` chart exposes an Ingress resource. Without a controller, the Ingress
object exists but does nothing.

```bash
# Docker Desktop / generic cluster
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.10.0/deploy/static/provider/cloud/deploy.yaml

# Verify the controller pod is Running
kubectl get pods -n ingress-nginx
```

---

## Chart Structure

```
deployment/helm/
├── README.md                   ← you are here
│
├── obs-infra/                  ← Chart 1: stateful infrastructure
│   ├── Chart.yaml              ← chart metadata (name, version, description)
│   ├── values.yaml             ← default configuration values
│   ├── .helmignore             ← files Helm should not package (like .gitignore)
│   ├── charts/                 ← sub-charts (dependencies) — empty for now
│   └── templates/              ← Go-template YAML files
│       ├── namespace.yaml
│       ├── kafka/
│       │   ├── statefulset.yaml
│       │   ├── service.yaml
│       │   └── pvc.yaml
│       ├── timescaledb/
│       │   ├── statefulset.yaml
│       │   ├── service.yaml
│       │   └── pvc.yaml
│       └── redis/
│           ├── deployment.yaml
│           └── service.yaml
│
└── obs-pipeline/               ← Chart 2: application services
    ├── Chart.yaml
    ├── values.yaml
    ├── charts/
    └── templates/
        ├── ingestor/
        ├── enricher/
        ├── detector/
        ├── query-api/
        ├── dashboard/
        ├── rbac/
        ├── networkpolicy/
        └── hpa/
```

Every file under `templates/` is a **Go template**. Helm renders it by substituting
`{{ .Values.xxx }}` placeholders with values from `values.yaml` (or override files),
then sends the resulting plain YAML to the Kubernetes API.

---

## Two-Chart Design

### Why not one chart for everything?

The infrastructure tier (Kafka, TimescaleDB, Redis) and the application tier have
**fundamentally different release lifecycles**:

| Concern | Infra | Apps |
|---|---|---|
| How often does it change? | Rarely (months) | Often (every PR) |
| What triggers a redeploy? | Config/version changes | Every new Docker image |
| Risk of data loss on upgrade? | High (stateful) | Low (stateless) |
| Who owns it? | Platform/SRE team | Dev team |

If everything were in one chart, every `helm upgrade` to deploy a new ingestor binary
would also touch Kafka — risking a rolling restart of a stateful broker for no reason.

Separating them gives you:

```bash
# Ship a new ingestor: fast, low-risk
helm upgrade obs-pipeline deployment/helm/obs-pipeline --set ingestor.image.tag=v2.3.1

# Upgrade Kafka: deliberate, planned maintenance window
helm upgrade obs-infra deployment/helm/obs-infra --set kafka.image=confluentinc/cp-kafka:8.3.0
```

This is the same principle as separating database migrations from application deploys in
a traditional CI/CD pipeline.

---

## Quick Start

These four commands bring the entire pipeline up on a local Kubernetes cluster:

```bash
# 1. Create the namespace (if not created by the chart itself)
kubectl create namespace obs-pipeline --dry-run=client -o yaml | kubectl apply -f -

# 2. Deploy infrastructure first
helm install obs-infra deployment/helm/obs-infra \
  --namespace obs-pipeline \
  --create-namespace

# 3. Wait for Kafka and TimescaleDB to be ready
kubectl rollout status statefulset/kafka -n obs-pipeline
kubectl rollout status statefulset/timescaledb -n obs-pipeline

# 4. Deploy the application tier
helm install obs-pipeline deployment/helm/obs-pipeline \
  --namespace obs-pipeline
```

To verify everything is running:

```bash
kubectl get pods -n obs-pipeline
kubectl get services -n obs-pipeline
```

---

## Key Concepts

### values.yaml — single source of truth

Every hardcoded value in a raw Kubernetes YAML (image tag, storage size, replica count,
port number, environment variable) becomes a named entry in `values.yaml`. Templates
reference it with `{{ .Values.<key> }}`.

**values.yaml** (defaults — works for local dev):
```yaml
kafka:
  image: confluentinc/cp-kafka:8.2.0
  storage: 1Gi

timescaledb:
  storage: 2Gi
```

**production-values.yaml** (overrides for prod):
```yaml
kafka:
  storage: 50Gi

timescaledb:
  storage: 200Gi
```

Deploy with the override:
```bash
helm install obs-infra deployment/helm/obs-infra -f production-values.yaml
```

The templates are identical. Only the values differ.

### Templating

Helm templates are standard Kubernetes YAML with Go `text/template` directives embedded.

The most common patterns you will see in this chart:

```yaml
# Reference a value
image: {{ .Values.kafka.image }}

# Conditional block — only render if value is set
{{- if .Values.kafka.storage }}
storage: {{ .Values.kafka.storage }}
{{- end }}

# Named template (defined in _helpers.tpl, called anywhere)
labels:
  {{- include "obs-infra.labels" . | nindent 4 }}

# Range — loop over a list
env:
{{- range .Values.kafka.env }}
  - name: {{ .name }}
    value: {{ .value | quote }}
{{- end }}
```

The `{{-` (dash) strips leading whitespace/newlines before the directive. This matters
because Kubernetes YAML is indentation-sensitive and a stray blank line can cause a
parse error.

### Releases and upgrades

When you run `helm install obs-infra ...`, Helm records a **release** named `obs-infra`
in the cluster (stored as a base64-encoded Secret in the target namespace). The release
tracks every revision.

```bash
# See all releases
helm list -n obs-pipeline

# See revision history of a release
helm history obs-infra -n obs-pipeline

# Upgrade to a new chart version or changed values
helm upgrade obs-infra deployment/helm/obs-infra -n obs-pipeline --set kafka.storage=5Gi

# Roll back to the previous revision
helm rollback obs-infra -n obs-pipeline

# Roll back to a specific revision number
helm rollback obs-infra 2 -n obs-pipeline
```

Each upgrade increments the revision counter. `helm rollback` creates a new revision
that applies the old configuration — it does not rewrite history.

### Dry run and diff

Before applying changes to a live cluster, preview what Helm would generate:

```bash
# Render templates to stdout without sending to Kubernetes
helm install obs-infra deployment/helm/obs-infra --dry-run --debug -n obs-pipeline

# Show a diff between the current release and pending upgrade (requires helm-diff plugin)
helm plugin install https://github.com/databus23/helm-diff
helm diff upgrade obs-infra deployment/helm/obs-infra -n obs-pipeline
```

`--dry-run` is invaluable for catching template rendering errors before they reach the
cluster.

### _helpers.tpl

Every chart can have a `templates/_helpers.tpl` file that defines **named templates**
(reusable snippets). Helm does not send `_helpers.tpl` to Kubernetes — it is only used
internally by other templates.

A typical helper defines standard labels so every resource in the chart gets consistent
metadata:

```yaml
{{- define "obs-infra.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}
```

Called in any template as:
```yaml
metadata:
  labels:
    {{- include "obs-infra.labels" . | nindent 4 }}
```

The `.` passes the current context (values, release info, chart info) into the helper.

---

## Useful Commands

| Command | What it does |
|---|---|
| `helm install obs-infra deployment/helm/obs-infra -n obs-pipeline` | Deploy infra for the first time |
| `helm install obs-pipeline deployment/helm/obs-pipeline -n obs-pipeline` | Deploy apps for the first time |
| `helm upgrade obs-infra deployment/helm/obs-infra -n obs-pipeline` | Apply chart or values changes to infra |
| `helm upgrade obs-pipeline deployment/helm/obs-pipeline -n obs-pipeline` | Apply chart or values changes to apps |
| `helm upgrade obs-pipeline ... --set ingestor.image.tag=v2` | Deploy a new image tag |
| `helm list -n obs-pipeline` | List all releases in the namespace |
| `helm history obs-infra -n obs-pipeline` | Show revision history for a release |
| `helm rollback obs-infra -n obs-pipeline` | Roll back infra to the previous revision |
| `helm uninstall obs-infra -n obs-pipeline` | Remove all resources created by the infra chart |
| `helm lint deployment/helm/obs-infra` | Validate chart structure and templates |
| `helm template obs-infra deployment/helm/obs-infra` | Render templates to stdout (no cluster needed) |
| `helm install obs-infra . --dry-run --debug` | Preview full render + API submission (no changes made) |
| `helm get values obs-infra -n obs-pipeline` | Show values used in the current release |
| `helm get manifest obs-infra -n obs-pipeline` | Show rendered manifests currently deployed |

---

## Environment-Specific Values

A practical pattern for managing multiple environments without duplicating charts:

```
deployment/helm/
├── obs-infra/          ← chart (never environment-specific)
├── obs-pipeline/       ← chart (never environment-specific)
└── envs/
    ├── dev-values.yaml
    ├── staging-values.yaml
    └── prod-values.yaml
```

```bash
# Dev deploy (small resources, single replica)
helm install obs-infra deployment/helm/obs-infra -f deployment/helm/envs/dev-values.yaml

# Prod deploy (large storage, multiple replicas)
helm install obs-infra deployment/helm/obs-infra -f deployment/helm/envs/prod-values.yaml
```

Multiple `-f` flags are also supported and merged left-to-right, so you can layer values:

```bash
helm install obs-infra deployment/helm/obs-infra \
  -f deployment/helm/envs/prod-values.yaml \
  -f deployment/helm/envs/prod-secrets.yaml   # secrets injected by CI, not committed
```

---

## Troubleshooting

### Pod stuck in `Pending`

Usually a PersistentVolumeClaim that cannot be satisfied. Check:

```bash
kubectl describe pod <pod-name> -n obs-pipeline
kubectl get pvc -n obs-pipeline
kubectl get storageclass
```

On Docker Desktop the default StorageClass is `hostpath`. Make sure it exists.

### `helm install` fails with "release already exists"

The release name is already recorded. Either upgrade it or uninstall first:

```bash
helm upgrade obs-infra deployment/helm/obs-infra -n obs-pipeline
# or
helm uninstall obs-infra -n obs-pipeline && helm install obs-infra ...
```

### Template rendering error

Run with `--dry-run --debug` to see the full rendered output and the exact line that
caused the error:

```bash
helm install obs-infra deployment/helm/obs-infra --dry-run --debug -n obs-pipeline 2>&1 | head -80
```

### ImagePullBackOff

The image tag in `values.yaml` does not exist in the registry. Check the tag spelling
and verify the image exists:

```bash
docker pull confluentinc/cp-kafka:8.2.0   # test the pull locally
```
