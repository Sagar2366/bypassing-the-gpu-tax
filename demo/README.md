# Live demo: Layer-7 routing between a CPU SLM pool and a GPU LLM pool

Talk: *Bypassing the GPU Tax* — WeAreDevelopers India, 26 Nov 2026, Stage 2, 15:20–15:50.

One endpoint, `POST /v1/chat/completions`. An Envoy ext_proc classifier reads the prompt, sets `x-route-tier`
(`slm` or `llm`), rewrites `model`, and a Gateway API `HTTPRoute` sends the request to:

| Tier | Backend | Notes |
|---|---|---|
| `slm` | **Ollama running `llama3.2:1b` on CPU** (real) | answers are real |
| `llm` | **mock "GPU" pool** (python, 1.5 s delay, $0.015/1K tokens) | stands in for vLLM on a GPU |

The default is `llm`. Unknown intent, tools, long context, risky topics and routing-injection attempts all go to `llm`.
Client-supplied `x-route-*` headers are stripped before classification.

```
client ─▶ Envoy Gateway ──ext_proc (gRPC)──▶ classifier (Go)
              │   sets x-route-tier, rewrites model, clears route cache
              ├─ x-route-tier=slm ─▶ Service/ollama     (CPU, llama3.2:1b)
              └─ default ──────────▶ Service/mock-llm   (GPU stand-in)
```

## What was and was not tested

Tested in my sandbox: the classifier (Go unit tests), and the whole data path with a **real Envoy 1.40.2** binary
using `scripts/local-envoy-test.yaml` (same ext_proc settings): correct pool per prompt, model rewrite, forged
headers overridden, injection routed to `llm`, decision echoed in response headers.

**Not tested** (no Docker/kind/helm in the sandbox): `setup.sh`, the Kubernetes manifests, Envoy Gateway v1.10.0's
`EnvoyExtensionPolicy`, and Ollama in the cluster. They follow the Envoy Gateway v1.10 docs, but **rehearse the full
path at least once, ideally two days before**. If `preflight.sh` fails and you can't fix it in 5 minutes, use the
fallback below.

## Before the talk

```bash
cd demo
scripts/setup.sh            # day before; kind + Envoy Gateway v1.10.0 + images + model (~5-10 min)
scripts/port-forward.sh     # terminal A, leave running  -> http://localhost:8080
scripts/preflight.sh        # 30 min before, and again 5 min before; must print PREFLIGHT PASSED
```

Terminal layout (large font, dark theme): **A** port-forward (can be hidden), **B** client, **C** classifier log:

```bash
kubectl logs -f deploy/classifier        # C: one JSON line per decision
```

Pre-load these tabs so you can recover quickly: `k8s/30-route.yaml`, `k8s/40-extproc.yaml`, `classifier/main.go`.

## Live script (about 6 minutes; slides 13–16)

1. **Show the route (slide 14).** `cat k8s/30-route.yaml`. Point out: header match to Ollama, catch-all to the big pool.
   "The route knows nothing about prompts. It only reads a header."
2. **Show the policy (slide 15).** `cat k8s/40-extproc.yaml`. Point out `failOpen: true` and `body: Buffered`.
3. **Run the mixed workload.** In B: `python3 scripts/demo-client.py`
   Expected: classify/extract/summarise/rewrite/translate → `slm`, about 0.3-3 s each (real CPU inference, first call
   after a cold start is slower); design/investigate/legal/vague → `llm`, about 1.5 s. The cost line at the end is
   illustrative: 5 of 9 prompts are SLM-eligible, which is *higher than a real workload*. Say so, and point back to slide 6.
4. **Show the log in C** while it runs: tier, reason, confidence, model per request.
5. **Spoof test (slide 16).** `python3 scripts/demo-client.py --spoof`
   A forged `x-route-tier: llm` on a simple prompt still goes to `slm`; a forged `slm` on a hard prompt still goes to `llm`.
6. **Injection test.** `python3 scripts/demo-client.py --inject`
   Expected `llm` / `routing-injection-suspected`.
7. **Failure drill (optional, 60 s, impressive).**
   `kubectl scale deploy/classifier --replicas=0`, then run the client: everything lands on `llm`
   (fail-open to the safe default, nothing breaks, you just pay more). Then `kubectl scale deploy/classifier --replicas=1`.
   Note: the first requests may take up to `messageTimeout` (2 s) each while Envoy notices the classifier is gone.

Closing line: the classifier is a few hundred lines of rules you can read in `classifier/main.go`; swap in a small ML
model or the vLLM Semantic Router behind the same ext_proc contract (slide 21).

## Fallback A: no cluster, real Envoy on the laptop (2 minutes)

Needs `go`, `envoy` and `python3`. Uses mock pools for both tiers (or `OLLAMA=1` to use a local Ollama on :11434).

```bash
ENVOY=$(which envoy) scripts/local-demo.sh     # terminal A, gateway on http://127.0.0.1:18080
python3 scripts/demo-client.py http://127.0.0.1:18080
```

## Fallback B: no infrastructure at all

Run `go test ./classifier/...` (shows the routing table-test) and walk through `classifier/main.go`. Have a screenshot
of a good run of `demo-client.py` saved on your desktop.

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `preflight`: policy not accepted | `kubectl describe envoyextensionpolicy tier-classifier`; check `kubectl get gateway eg` is `Programmed` and field names against the v1.10 docs (`failOpen`, `messageTimeout`, `processingMode`) |
| Everything routes to `llm`, log is empty | ext_proc is not attached or classifier unreachable: `kubectl logs deploy/classifier`; `kubectl get svc classifier` must expose h2c/gRPC on 9002 |
| `502`/`503` from gateway | backend not ready: `kubectl get pods`; Ollama may still be loading the model |
| First SLM call very slow | model cold start; `preflight.sh` warms it, `OLLAMA_KEEP_ALIVE=24h` keeps it hot |
| `ImagePullBackOff` | images were not side-loaded; re-run `setup.sh` on good wifi (`kind load docker-image ...`) |
| Port 8080 busy | edit the port in `port-forward.sh` and pass the URL to `demo-client.py` |
| Response lacks `x-route-*` headers | response processing not enabled; the client still works and falls back to `?`/`ollama-cpu`. Check `processingMode.response` |

## Cleanup

`scripts/teardown.sh` deletes the kind cluster.

## Files

`classifier/` Go ext_proc server, tests, Dockerfile · `mock-llm/server.py` GPU-pool stand-in ·
`k8s/` manifests (numbered in apply order) · `scripts/` setup, port-forward, preflight, demo client, teardown, local fallback.
