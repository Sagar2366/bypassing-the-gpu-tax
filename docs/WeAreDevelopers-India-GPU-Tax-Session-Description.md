# Session description: WeAreDevelopers India (KubeCon-style)

**Speaker:** Sagar Utekar, SRE, CrowdStrike
**Session type:** Talk, 30 minutes (26 Nov 2026, 15:20-15:50, Stage 2)
**Level:** Intermediate
**Track:** AI infrastructure / Platform engineering / Cloud native

## Title

Bypassing the GPU Tax: Smart Layer-7 Routing with Small Language Models

### Alternatives

1. Not Every Prompt Needs a Frontier Model: Routing Agent Steps to CPU-Hosted SLMs with Envoy
2. Route by Header, Pay by Need: An Open-Source Blueprint for Cheaper LLM Traffic
3. The Cheapest Token Is the One the GPU Never Sees

## Abstract

Most LLM applications send every request to the biggest model on the biggest GPU cluster, including the many small steps an agent produces: classify, extract, reformat, rewrite. Those steps are bounded and easy to check, and many can be served by small language models under 8B parameters on ordinary CPU nodes. This talk shows how to divert them without touching application code. We build a three-layer design from open-source parts: Envoy Proxy and the Kubernetes Gateway API intercept requests at Layer 7, a lightweight classifier attached through ext_proc sets a routing header, and the Gateway sends traffic to a CPU pool of SLMs or a GPU pool running vLLM. We work through the savings maths, including the case where router overhead or a harder traffic mix erases most of the gain, and cover the failure modes: under-routing, prompt-injected routing and model drift. You leave with a routing schema, Gateway API and Envoy configuration blueprints, and a safe rollout plan based on shadow mode, golden sets and canaries.

## Key takeaways for attendees

- How to estimate savings from the share of agent steps you can safely divert, and why the result ranges from about 3x to about 9x depending on traffic mix
- How Envoy ext_proc and Gateway API header matching give portable L7 model routing, with a fail-safe default to the strong model
- When to route up front and when to cascade, and why self-reported model confidence is not a safe gate
- How to measure cost per successful task, escalation rate and first-pass acceptance instead of token price alone
- A four-step rollout: log, shadow, evaluate, canary

## Benefits to the ecosystem

Teams are adopting agents faster than they are building cost controls. This session offers a vendor-neutral, infrastructure-level answer built on CNCF-adjacent projects (Envoy, Gateway API) and open model runtimes (vLLM, Ollama), and it complements eval and gateway-governance tooling rather than replacing it. Every figure is labelled as illustrative, reported by a named source, or a claim to test on your own traffic.

## Notes for the review committee

- Includes config blueprints (routing schema, HTTPRoute, EnvoyExtensionPolicy ext_proc) to verify against current versions.
- Not a product pitch; the open-source landscape slide covers vLLM Semantic Router, the Gateway API Inference Extension and Agent Router, and says when to adopt them instead.
- Sources: ByteByteGo (Sep 2026), Spacestem (2026), project documentation, and a related WeAreDevelopers North America talk by Viktoria Semaan, credited on the References slide.
