#!/usr/bin/env bash
# Run 30 min before going on stage (and again right before). Warms the model and checks all four routes.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
BASE=${BASE:-http://localhost:8080}
ok(){ echo "  [ok]   $*"; }; bad(){ echo "  [FAIL] $*"; FAIL=1; }; FAIL=0
echo "pods:"; kubectl get pods -o wide | sed 's/^/  /'
for d in ollama mock-llm classifier; do
  kubectl rollout status deploy/$d --timeout=20s >/dev/null 2>&1 && ok "deploy/$d ready" || bad "deploy/$d not ready"
done
kubectl exec deploy/ollama -- ollama list 2>/dev/null | grep -q llama3.2 && ok "model present" || bad "model missing: kubectl exec deploy/ollama -- ollama pull llama3.2:1b"
kubectl get envoyextensionpolicy tier-classifier -o jsonpath='{.status.ancestors[0].conditions[?(@.type=="Accepted")].status}' 2>/dev/null | grep -q True \
  && ok "EnvoyExtensionPolicy accepted" || bad "EnvoyExtensionPolicy not accepted: kubectl describe envoyextensionpolicy tier-classifier"
curl -s -o /dev/null --max-time 3 "$BASE/" ; [ $? -eq 7 ] && bad "gateway not reachable at $BASE (start scripts/port-forward.sh)"
echo "warm-up (first Ollama call loads the model, can take 10-30 s):"
python3 "$HERE/demo-client.py" "$BASE" --inject | tail -2
curl -s --max-time 120 -o /dev/null "$BASE/v1/chat/completions" -H 'content-type: application/json' \
  -d '{"model":"auto","messages":[{"role":"user","content":"Extract the number from: abc 42"}]}' && ok "SLM path warmed" || bad "SLM path failed"
echo "full dry run:"; python3 "$HERE/demo-client.py" "$BASE"
[ $FAIL -eq 0 ] && echo "PREFLIGHT PASSED" || { echo "PREFLIGHT HAS FAILURES (see above, or use scripts/local-demo.sh as fallback)"; exit 1; }
