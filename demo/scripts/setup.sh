#!/usr/bin/env bash
# One-time cluster bring-up. Run it the day BEFORE the talk (pulls images + the model). ~5-10 min.
# Needs: docker, kind, kubectl, helm.
set -euo pipefail
DEMO="$(cd "$(dirname "$0")/.." && pwd)"
EG_VERSION=${EG_VERSION:-v1.10.0}
MODEL=${MODEL:-llama3.2:1b}
for b in docker kind kubectl helm; do command -v $b >/dev/null || { echo "missing: $b"; exit 1; }; done

echo "==> kind cluster"
kind get clusters | grep -qx wad-demo || kind create cluster --config "$DEMO/k8s/kind-config.yaml"
kubectl config use-context kind-wad-demo

echo "==> Envoy Gateway $EG_VERSION"
helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm --version "$EG_VERSION" \
  -n envoy-gateway-system --create-namespace --wait
kubectl wait -n envoy-gateway-system deploy/envoy-gateway --for=condition=Available --timeout=5m

echo "==> classifier image"
docker build -t wad/classifier:demo "$DEMO/classifier"
kind load docker-image wad/classifier:demo --name wad-demo

echo "==> pre-pull other images into kind (so stage wifi does not matter)"
for img in ollama/ollama:latest python:3.12-slim; do
  docker pull "$img"; kind load docker-image "$img" --name wad-demo
done

echo "==> manifests"
kubectl create configmap mock-llm-src --from-file=server.py="$DEMO/mock-llm/server.py" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f "$DEMO/k8s/00-gateway.yaml" -f "$DEMO/k8s/10-backends.yaml" -f "$DEMO/k8s/20-classifier.yaml"
kubectl rollout status deploy/ollama deploy/mock-llm deploy/classifier --timeout=5m 2>/dev/null || \
  for d in ollama mock-llm classifier; do kubectl rollout status deploy/$d --timeout=5m; done

echo "==> pull $MODEL into Ollama (stored on a PVC, survives pod restarts)"
kubectl exec deploy/ollama -- ollama pull "$MODEL"

kubectl apply -f "$DEMO/k8s/30-route.yaml" -f "$DEMO/k8s/40-extproc.yaml"
kubectl wait gateway/eg --for=condition=Programmed --timeout=5m
echo "Done. Next: scripts/preflight.sh"
