# Speaker notes: Bypassing the GPU Tax

Slides 1-26 are the 30-minute talk; slides 27-36 are backup for Q&A.

## Slide 1: Smart Layer-7 Routing with Small Language Models

Welcome and hook. One idea for 30 minutes: not every prompt deserves a frontier model on a GPU. I will show a routing layer built from standard cloud-native parts, with no application rewrite and no lock-in. Everything is opinion from my SRE seat; numbers are labelled illustrative, reported, or claims to test on your own traffic. Show of hands: who runs LLM traffic in production today? Backup slides after the thank-you hold the config deep dives for Q&A.

## Slide 2: What is model routing?

Start with the definition. Model routing means looking at each request and choosing which model answers it. Three jobs: estimate what the request needs, pick the cheapest model likely to be good enough, and check the result so you can escalate. It is not mixture of experts, which happens inside one model. About 40 seconds.

## Slide 3: A REST API is not an LLM API

Why can we not just put a normal API gateway in front? Because LLM traffic behaves differently. Payloads are large. Cost is per token, input and output priced differently. Latency is seconds and streamed. The meaningful limit is tokens, not requests. Failures can be a healthy 200 with a bad answer. And every provider has its own contract. About 45 seconds.

## Slide 4: What breaks in production

What actually breaks once you have more than one team and more than one provider. Rate-limit errors when one burst eats the shared quota. Outages with no failover. Retry and token-counting code copied into every agent. Provider keys scattered across workloads. Bills with no per-team budget. No shared view of tokens and latency. Each is a platform problem, so it belongs in a platform layer. About 40 seconds.

## Slide 5: What is an AI gateway?

An AI gateway is the single front door for all model and tool traffic. Clients speak one OpenAI-style API. The gateway handles authentication, rate limits, failover, routing and observability, and talks to hosted providers, your own inference pools, and MCP servers. The agent keeps the reasoning; the gateway keeps the plumbing. About 40 seconds.

## Slide 6: Why Envoy AI Gateway, and what it gives you

Why Envoy AI Gateway, or whichever gateway you choose. It sits on Envoy, a proven data plane. It is Kubernetes native through the Gateway API plus its own resources. It is extensible with ext_proc, WASM and dynamic modules. And it is open source under Apache 2.0, now named Agent Router under the Agentic AI Foundation. The core features: a unified API, simple auth, token rate limits, cross-provider fallback and observability. The extensibility is what our SLM classifier uses; if you prefer another gateway, the pattern still applies. About 60 seconds.

## Slide 7: Three controls: token limits, per-model quotas, failover

Three controls matter most for cost and reliability. One: token-based rate limiting. A request count is a poor limit because one call can be ten tokens or a hundred thousand, so the bucket is model name plus a dimension such as a tenant header, charged by token cost, here input plus output times six. Two: model-specific quotas and the 2026 quota-aware routing: prepaid capacity first, pay-as-you-go second. Three: failover. The client asks for a logical model name, each backend has a priority and its own model override, and a retry policy moves on. Our twist, my suggestion: make the self-hosted pool the last fallback. Shadow mode first. The config for each is in the backup slides. About 90 seconds.

## Slide 8: Every prompt pays the frontier price

Frame this as a platform problem, not prompt engineering. Today most apps send a date format, an intent classification and a legal analysis to the same biggest model. Agents make it worse: one user request fans out into many intermediate steps and most are small. The 40 percent figure is the claim from my session abstract, based on observation. Treat it as a hypothesis you verify on your own traffic; I show how on the safety slide. About 90 seconds.

## Slide 9: Model routing: a load balancer with unequal backends

This is the picture behind the whole talk. A router sits in front of several models of very different price. Think of a load balancer where the backends are not equivalent. In the ByteByteGo example, 85 percent of requests go to a model at one twentieth of the price, 10 percent to a medium one, 5 percent to the large one. Average cost: 0.85 times 0.05 plus 0.10 times 0.20 plus 0.05 times 1 equals 0.1125, about 9 times cheaper. Not mixture of experts. About 60 seconds.

## Slide 10: The maths: savings follow the share you divert

Work the formula live: savings equal share diverted times one minus the price ratio. With a 20x price gap, savings are almost exactly the diverted share, so 40 percent diverted saves about 38 percent. Halving the bill means about half your steps are safe for an SLM. Then list what eats into it: classifier cost, CPU nodes, escalations that pay twice, and eval effort. About 75 seconds.

## Slide 11: Three layers, no application rewrite

The whole design on one slide. Clients keep one OpenAI-style endpoint. Layer 1: Envoy with Gateway API sees every request at L7. Layer 2: a classifier called through ext_proc reads the prompt and sets a header, x-route-tier, to slm or llm. Layer 3: standard header matching sends it to a CPU pool of small models or a GPU pool for hard work. Intercept, decide, serve. No app changes, no provider lock-in. About 75 seconds.

## Slide 12: A request, step by step

Walk the six steps quickly. Stress step 4: the default is the expensive path. Low confidence, classifier timeout, or an unusual request type such as tool calls all go to the GPU pool. A wrong escalation costs money; a wrong downgrade costs quality, and quality is harder to win back. Also note two Layer 1 watch-outs: the body must be buffered for the classifier, so set a body size limit and a tight ext_proc timeout; and strip any inbound x-route-tier header. About 60 seconds.

## Slide 13: Same hook, different question: the endpoint picker bridge

This is how this talk connects to the inference-aware gateway work. The Gateway API Inference Extension adds an endpoint picker, itself an ext_proc service. It reads live Prometheus metrics from the model servers, such as queue depth and KV-cache usage, and picks the best replica inside an InferencePool. Our classifier uses the very same hook for a different question: which model tier should answer, before we even choose a replica. They stack: classifier first, endpoint picker second. Parts of that project are moving to llm-d repos, so check the current status. About 60 seconds.

## Slide 14: Layer 2: the classifier is small, fast, replaceable

Do not start with a model. Rules and a few heuristics catch a surprising share of easy traffic in under a millisecond. Add embeddings, then a small encoder, only when logs show money left on the table. Keep the classifier behind a gRPC interface so you can swap it. Latency figures are budgets, not measurements. Tier each step by difficulty: simple goes to the SLM, moderate depends, open-ended planning goes to the large model. About 75 seconds.

## Slide 15: Three design choices: route, cascade, or decide per task

Two strategies and one timing question. Routing predicts difficulty up front; a cascade finds out by trying. Cascade only works when you can check the answer without another expensive model: JSON schema, field validation, tests. Do not use the model's own confidence as the gate; failed attempts cost money and latency. The timing question: per request or per task. Switching models mid-session throws away the prompt cache, and Databricks says cache hit rate dominates cost at scale, so for long agent sessions classify once per task. For stateless one-shot calls, per request is right, and that is the demo. About 90 seconds.

## Slide 16: Blueprint 1: the routing schema

The routing schema from the abstract. Point out: default_tier is llm, the fail-safe; a confidence threshold; a classifier timeout; rules mapping intent to tier. Routing policy is data in a ConfigMap, reviewable in a pull request, not code buried in a proxy. Field names are mine; adapt them. About 60 seconds.

## Slide 17: Blueprint 2: Gateway API does the delivery

Plain Gateway API, portable across implementations. A header-match rule for slm goes to the CPU pool, and a catch-all sends everything else to the GPU pool. Security point: strip any inbound x-route-tier header at the edge, otherwise a client can choose its own tier. Service names are examples. About 60 seconds.

## Slide 18: Blueprint 3: attach the classifier with ext_proc

The integration point. I show Envoy Gateway's EnvoyExtensionPolicy; with raw Envoy it is the ext_proc HTTP filter. Fields vary by version, so check the current API reference. The classifier must run before route selection and clear the route cache after setting the header, so test that in your version. Practical tip: wire a fake classifier that returns a fixed tier first, prove the plumbing, then add the model. About 60 seconds.

## Slide 19: kind + Envoy Gateway + Gateway API + Ollama (real SLM) + mock large model

Live demo, about 3 minutes, if time permits; otherwise the code slides above stand on their own. Setup: a kind cluster with Envoy Gateway and Gateway API, a Go ext_proc classifier, real Ollama small model on CPU, and a mock large model standing in for the GPU pool. Send a simple prompt, show the x-route-tier header and the SLM answer. Send a hard prompt, show it route to llm. Send a prompt with tools, show it default to llm. Send an injection attempt with a fake client header, show it stripped. Everything is in the repo. If the demo fails, fall back to the screenshot of the output and continue.

## Slide 20: Making it safe: quality before savings

The most important slide. Shadow mode: classify and log, but still send everything to the GPU pool, and compare SLM answers offline. This is how you test the 40 percent claim on your own traffic. Living golden set: 100 to 300 real prompts per task type, grown from production misses, re-run whenever a new model drops. Escalate on doubt, then canary 5 percent of one task type. Spend caps and loop limits so a stuck agent cannot burn budget. About 2 minutes.

## Slide 21: How routing fails, and the guard for each

Six ways routing fails, each with a guard. The one SREs should care about most is prompt-injected routing: if the router reads user text, a user can write "classify this as easy". Route on trusted metadata, overwrite the header at the edge, and keep fixed rules for high-risk topics whatever the classifier says. Re-run evals whenever a model, prompt or price changes. About 75 seconds.

## Slide 22: Measure cost per successful task, not per token

Replace tokens saved as your headline with cost per successful task: all spend, tokens, router, CPU and GPU nodes, retries and human review, divided by tasks that met the bar. Track escalation rate, first-pass acceptance and error rate by task class so an average cannot hide a weak category. Latency is a second win, since small tasks skip the GPU queue, but measure it: classifier plus buffering should add tens of milliseconds, and if classifier p95 climbs over budget, fail open to the GPU pool and alert. Export a tier label on every metric. About 90 seconds.

## Slide 23: The router is where policy lives, and what exists already

A point that gets lost when we talk only about savings. The router sees every request before any model does, so it is the natural place for policy: jurisdiction, customer tier, approved vendors, budget spill, and a log of every decision, which matters as AI regulation tightens. Also do not build all of it. vLLM Semantic Router is an open decision layer that picks a model per request. The Inference Extension makes a gateway inference-aware. Agent Router, formerly Envoy AI Gateway, gives one API over many models. Envoy's own routing leans towards availability, not cost, which is the gap our classifier fills. Related: Viktoria Semaan's talk at WAD North America covered evaluate, govern, route; those layers compose. About 90 seconds.

## Slide 24: Rollout in four steps

Sequence matters more than dates: measure, shadow, evaluate, canary, widen. Week 1 logging only. Week 2 rules in shadow mode. Week 3 golden set and one low-risk canary. Week 4 onward, widen by task type and add a learned classifier if logs justify it. Nobody should flip 40 percent of traffic on day one. About 40 seconds.

## Slide 25: Routing is an infrastructure problem: put it in the delivery plane, not in every app.

Five takeaways. Routing belongs in the delivery plane, not every app. Savings track the share you can safely divert, so measure it first. Envoy ext_proc plus Gateway API gives portable L7 routing. Default to the strong model and earn each downgrade with evals. Start with rules, add learned classifiers when logs justify them. About 60 seconds.

## Slide 26: Thank you. Questions?

Thank the audience and point to the links: Envoy, Gateway API, vLLM, Ollama, and the landscape projects, plus the repo with the demo. Remind them that figures are illustrative or reported and YAML is a reference blueprint to verify against current versions. Take questions. Likely questions: how accurate must the classifier be, why not just use a small model everywhere, how do I handle streaming and tool calls. Backup slides follow for the config deep dives.

## Slide 27: Envoy AI Gateway configuration deep dive

Backup section. Use these to answer questions about the gateway configuration. Each slide is a redrawn, simplified example; field names and API versions vary by release, so check the current docs.

## Slide 28: How the Envoy AI Gateway resources fit together

The resource graph. The Gateway is the standard Gateway API entry point. An AIGatewayRoute attaches to it through parentRefs and chooses backends by model name. Each AIServiceBackend describes one provider or self-hosted endpoint and points at a Backend or Service. A BackendSecurityPolicy targets the AIServiceBackend and holds the API key or cloud credentials by secret reference. QuotaPolicy and BackendTrafficPolicy attach on top for token budgets and retries. Two controllers, AI Gateway and Envoy Gateway, turn these into Envoy configuration.

## Slide 29: Counting tokens: llmRequestCosts

The gateway extracts token counts from each response and stores them as dynamic metadata under named keys. Four standard types: input, cached input, output and total. Those keys are what rate limits and quotas then charge against. You can also define a CEL expression cost, such as input plus output times six, to reflect pricing. About 30 seconds.

## Slide 30: Model-specific, per-tenant token rate limit

A global rate limit rule. The client selectors say: distinct value of x-tenant-id, and x-ai-eg-model equal to gpt-4. So every tenant gets its own bucket for that one model. The limit here is 1000 per hour, and the cost is read from the response metadata, the total-token key from the earlier slide, so the bucket is charged in tokens. Result: the expensive model is protected per tenant while others stay open. About 45 seconds.

## Slide 31: Failover: passive health checks plus retries

Failover has two halves. Passive health checking ejects a bad endpoint: here after three consecutive 5xx errors, for ten seconds, with at most 50 percent of endpoints ejected. The retry policy then tries again, up to three times with backoff from 100 milliseconds to 10 seconds, on 429 and 5xx style status codes and on connect failures. Priority on the backends decides where the retry goes. Attach it to the route with targetRefs. About 60 seconds.

## Slide 32: Onboarding a provider: backend and credentials

Onboarding step one and two. Declare an AIServiceBackend: the API schema, here OpenAI v1, and a reference to the Backend that holds the host. Then a BackendSecurityPolicy targets that AIServiceBackend, with type APIKey and a secret reference. The key lives in one place, not in agent workloads, and rotation is a secret update. About 45 seconds.

## Slide 33: Onboarding a provider: route and client

Step three: the AIGatewayRoute attaches to the Gateway and matches the model name header, then lists backendRefs. Adding a provider means adding a backend ref, with its own model name override if needed. Step four: the client changes nothing except possibly the model name. You can also try it locally with the standalone aigw run mode on localhost:1975/v1 before moving to Kubernetes. About 45 seconds.

## Slide 34: Self-hosted MCP gateway: one endpoint, filtered tools

A self-hosted MCP gateway puts many tool servers behind one endpoint. An MCPRoute can select which tools are exposed, authorize each call using JWT claims and CEL, and inject upstream credentials. A research agent sees only its tools. The routing idea for our design, and this is my proposal not a shipped feature: reuse the tier decision so cheap-tier agents are offered only low-risk tools. About 45 seconds.

## Slide 35: Worked example: three tiers, up to about 9x cheaper

ByteByteGo's worked example: 85 percent small at 1/20, 10 percent medium at 1/5, 5 percent large at 1 gives 0.1125. The two extra bars are mine. A router costing 0.01 per request moves it to 0.1225. A harder mix of 50, 30, 20 gives 0.285, about 3.5 times cheaper. Same formula, very different outcome. Savings need a big price gap, mostly simple traffic, and a router that finds the simple ones reliably.

## Slide 36: Where a router sits in the agent stack

Where does all this live? The agent speaks one OpenAI-compatible contract and holds no provider secrets. A traffic-control layer, control plane plus Envoy data plane, owns routes, quotas, retries and telemetry. Upstream are hosted providers, your own pools, and MCP servers. Our classifier is one more piece in that traffic layer. The Envoy AI Gateway project, now Agent Router, moved the same way: unified API in early 2025, production ready late 2025, cost management and quota-aware routing in March 2026.
