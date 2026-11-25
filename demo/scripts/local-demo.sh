#!/usr/bin/env bash
# Laptop-only fallback: classifier + mock pools + a local Envoy. No Docker/kind needed.
# Needs: go, envoy on PATH (or ENVOY=/path), python3. Optionally a real Ollama on :11434 (OLLAMA=1).
set -euo pipefail
DEMO="$(cd "$(dirname "$0")/.." && pwd)"
ENVOY=${ENVOY:-envoy}
mkdir -p /tmp/wad-demo && cd /tmp/wad-demo
(cd "$DEMO/classifier" && go build -o /tmp/wad-demo/classifier .)
SLM_PORT=18001; [ "${OLLAMA:-0}" = 1 ] && SLM_PORT=11434
LISTEN=:19002 ./classifier >classifier.log 2>&1 &
[ "${OLLAMA:-0}" = 1 ] || POOL_NAME=slm-cpu-pool DELAY_SECONDS=0.3 USD_PER_1K_TOKENS=0.00075 PORT=18001 python3 "$DEMO/mock-llm/server.py" >slm.log 2>&1 &
POOL_NAME=llm-gpu-pool DELAY_SECONDS=1.5 PORT=18002 python3 "$DEMO/mock-llm/server.py" >llm.log 2>&1 &
sed "s/port_value: 18001/port_value: $SLM_PORT/" "$DEMO/scripts/local-envoy-test.yaml" > envoy.yaml
sleep 1
echo "Gateway on http://127.0.0.1:18080  (Ctrl-C to stop)"
trap 'kill $(jobs -p) 2>/dev/null' EXIT
"$ENVOY" -c envoy.yaml --log-level warn
