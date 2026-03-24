NAMESPACE = obs-pipeline

.PHONY: build-images deploy-infra deploy-app deploy-all teardown status logs

# Build all 5 Docker images.
# Run from repo root (k8s/../ = repo root).
# imagePullPolicy: IfNotPresent in each Deployment means k8s uses these local images.
build-images:
	docker build -t obs-ingestor:latest -f ../services/ingestor/Dockerfile ..
	docker build -t obs-enricher:latest -f ../services/enricher/Dockerfile ..
	docker build -t obs-detector:latest ../services/detector/
	docker build -t obs-query-api:latest ../services/query-api/
	docker build -t obs-dashboard:latest ../services/dashboard/

# Deploy infrastructure: Kafka, TimescaleDB, Redis + their init jobs.
# Uses kubectl wait to block until each Job completes before moving on.
deploy-infra:
	kubectl apply -f namespace.yaml
	kubectl apply -f infra/kafka/
	kubectl rollout status statefulset/obs-kafka -n $(NAMESPACE) --timeout=180s
	kubectl apply -f infra/kafka-init-job.yaml
	kubectl wait --for=condition=complete job/kafka-init -n $(NAMESPACE) --timeout=120s
	kubectl apply -f infra/timescaledb/
	kubectl rollout status statefulset/timescaledb -n $(NAMESPACE) --timeout=120s
	kubectl apply -f infra/db-init-job.yaml
	kubectl wait --for=condition=complete job/db-init -n $(NAMESPACE) --timeout=120s
	kubectl apply -f infra/redis/
	kubectl rollout status deployment/redis -n $(NAMESPACE) --timeout=60s

# Deploy app services + Ingress.
# ConfigMap and Secret first so pods can read env vars on startup.
deploy-app:
	kubectl apply -f manifests/configmap.yaml -f manifests/secret.yaml
	kubectl apply -f manifests/ingestor/
	kubectl apply -f manifests/enricher/
	kubectl apply -f manifests/detector/
	kubectl apply -f manifests/query-api/
	kubectl apply -f manifests/dashboard/
	kubectl apply -f ingress.yaml

deploy-all: deploy-infra deploy-app

# Tear everything down. || true so make doesn't fail if resources don't exist.
teardown:
	kubectl delete namespace $(NAMESPACE) || true

# Show all resources in the namespace at a glance.
status:
	kubectl get all -n $(NAMESPACE)

# Stream logs from a specific service. Usage: make logs SVC=ingestor
logs:
	kubectl logs -n $(NAMESPACE) deployment/$(SVC) -f
