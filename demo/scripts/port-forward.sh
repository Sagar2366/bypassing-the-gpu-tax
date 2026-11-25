#!/usr/bin/env bash
# Expose the Envoy Gateway data plane on http://localhost:8080 (leave this running in its own terminal).
set -euo pipefail
SVC=$(kubectl get svc -n envoy-gateway-system \
  --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=eg \
  -o jsonpath='{.items[0].metadata.name}')
echo "forwarding svc/$SVC -> http://localhost:8080"
exec kubectl -n envoy-gateway-system port-forward "svc/$SVC" 8080:80
