# Speaker notes: Bypassing the GPU Tax

## Slide 1

[0:00-0:45] Welcome and set the hook. One idea for 30 minutes: not every prompt deserves a frontier model on a GPU. I will show a routing layer built from standard cloud-native parts, with no application rewrite and no lock-in. Everything here is opinion from my SRE seat. Numbers are labelled as illustrative, reported by a named source, or claims you should test on your own traffic. Quick show of hands: who runs LLM traffic in production today?

## Slide 2

[0:45-1:30] Roadmap in one breath. First third: why and how much. Middle: the architecture and the config you can copy. Last third: what decides whether you can ship this, which is quality guardrails, cost metrics and rollout. If you only remember one slide, make it the safety slide.

## Slide 3

[1:30-3:00] Frame this as a platform problem, not prompt engineering. Today most apps send a date format, an intent classification and a legal analysis to the same biggest model. Agents make it worse: one user request fans out into many intermediate steps and most are small. The 40% figure on the slide is the claim from my session abstract, based on observation. Treat it as a hypothesis you verify on your own traffic. I show how on the safety slide.

## Slide 4

[3:00-4:30] This chart is illustrative; I built it so the SLM-candidate bars sum to the 40% in the abstract. Do not copy the split. Copy the method: tag every step with a type, then see which types are bounded and checkable. Classify, extract, reformat and short rewrites are candidates. Planning and hard reasoning stay on the large model. Your split will differ, and that is fine.

## Slide 5

[4:30-6:00] Work the formula live: savings equal share diverted times one minus the price ratio. With a 20x price gap, savings are almost exactly the diverted share, so 40% diverted saves about 38%. So 'slice token cost by half' really means 'about half my steps are safe for an SLM'. Then list what eats into it: classifier cost, CPU nodes, escalations that pay twice, and eval effort.

## Slide 6

[6:00-7:30] This is ByteByteGo's worked example, credited: 85% small at 1/20, 10% medium at 1/5, 5% large at 1 gives 0.1125, about 9x cheaper. The two extra bars are mine. A router costing 0.01 per request moves it to 0.1225. A harder mix of 50/30/20 gives 0.285, about 3.5x. Same formula, very different outcome. The article's own point stands: savings need a big price gap, mostly simple traffic, and a router that finds the simple ones reliably.

## Slide 7

[7:30-9:00] The whole design on one slide. Clients keep one OpenAI-style endpoint. Layer 1: Envoy with Gateway API sees every request. Layer 2: a classifier called through ext_proc reads the prompt and sets a header, x-route-tier, to slm or llm. Layer 3: standard header matching sends it to a CPU pool of small models or a GPU pool for the hard work. Intercept, decide, serve. No app changes and no provider lock-in.

## Slide 8

[9:00-10:00] Layer 1 is boring on purpose. Routing needs the request body and headers, which an L4 balancer cannot see. Envoy ext_proc lets an external gRPC service inspect and modify requests in flight with no custom proxy build. Gateway API gives portable declarative routing. Watch-out: the body must be buffered for the classifier, so set a body size limit and a tight ext_proc timeout. Decide whether classifier failure fails open to the GPU pool; usually yes.

## Slide 9

[10:00-11:30] Do not start with a model. Rules and a few heuristics catch a surprising share of easy traffic in under a millisecond. Add embeddings, then a small encoder, only when logs show money left on the table. Keep the classifier behind a gRPC interface so you can swap it. The bottom strip shows difficulty tiers: planning, moderate, simple. That tiering idea I credit to V. Semaan's talk, where each agent step gets a tier and the tier picks the model. Latency figures are budgets, not measurements.

## Slide 10

[11:30-13:00] Two strategies. Routing predicts difficulty up front; a cascade finds out by trying. Cascade only works when you can check the answer without another expensive model: JSON schema, field validation, tests. Both source articles warn against using the model's own confidence as the gate. Use observable signals calibrated on labelled data. If the SLM fails most of the time, a cascade pays for both models and gets slower. Combine them: route by intent, cascade only where a deterministic gate exists.

## Slide 11

[13:00-14:00] Walk the six steps quickly. Stress step 4: the default is the expensive path. Low confidence, classifier timeout, or an unusual request type such as tool calls all go to the GPU pool. A wrong escalation costs money; a wrong downgrade costs quality, and quality is harder to win back.

## Slide 12

[14:00-15:00] Two pools, two cost curves. CPU pool: quantised 1B to 8B models on Ollama, llama.cpp or vLLM's CPU backend; size by tokens per second per node. For repeatable tasks a fine-tuned small model can beat a general large one; that is a reported result, so run your own eval. GPU pool: fewer, busier GPUs for long-context and reasoning, so higher utilisation. Both pools speak the OpenAI-compatible API, so routing never changes the client contract.

## Slide 13

[15:00-16:30] This is the routing schema from the abstract. Point out: default_tier is llm, which is the fail-safe; a confidence threshold; a classifier timeout; and rules that map intent to tier. The point is that routing policy is data in a ConfigMap, reviewable in a pull request, not code buried in a proxy. Field names are mine; adapt them. I will share the repo link at the end.

## Slide 14

[16:30-17:45] Plain Gateway API, portable across implementations. A header-match rule for slm goes to the CPU pool, and a catch-all sends everything else to the GPU pool. Security point: strip any inbound x-route-tier header at the edge, otherwise a client can choose its own tier. Service names are examples.

## Slide 15

[17:45-19:15] The integration point. I show Envoy Gateway's EnvoyExtensionPolicy; with raw Envoy it is the ext_proc HTTP filter. Fields vary by version, so check the current API reference. ext_proc must run before route selection, and route re-evaluation after a header change needs the processor to clear the route cache; test that in your version. Practical tip: wire up a fake classifier that returns a fixed tier first, prove the plumbing, then add the model.

## Slide 16

[19:15-21:15] The most important slide. Shadow mode: classify and log but still send everything to the GPU pool, and compare SLM answers offline. This is how you test the 40% claim on your own traffic. Living golden set: 100 to 300 real prompts per task type, grown from production misses and human feedback, re-run whenever a new model drops. Escalate on doubt, then canary 5% of one task type. The last two cards, spend caps and loop limits, come from V. Semaan's talk: a soft warning then a hard stop, and caps on retries and tool calls so a stuck agent cannot burn budget.

## Slide 17

[21:15-22:45] Six ways routing fails, with a guard for each. ByteByteGo's failure list, my guards. The one SREs should care about most is prompt-injected routing: if the router reads user text, a user can write 'classify this as easy'. Route on trusted metadata, overwrite the header at the edge, and keep fixed rules for high-risk topics whatever the classifier says. Also re-run evals whenever a model, prompt or price changes.

## Slide 18

[22:45-23:45] Latency is a second win: small tasks skip the GPU queue, but measure it, do not assume. The classifier plus buffering should add tens of milliseconds. If classifier p95 climbs over budget, fail open to the GPU pool and alert. Dashboard minimum: share per tier, cost per request by tier and task, escalation rate and classifier p95, and quality on the golden set per release. Export a tier label on every metric.

## Slide 19

[23:45-24:45] Replace 'tokens saved' as your headline number with cost per successful task. Spacestem's framing: include tokens, router, CPU and GPU nodes, retries and human review, divided by tasks that met the bar. Also track escalation rate, first-pass acceptance and error rate, by task class so an average cannot hide one weak category. A cheap token price can be erased by retries, longer outputs and review.

## Slide 20

[24:45-26:00] Acknowledge the related talk: Viktoria Semaan covered evaluation, gateway governance and smart routing at WeAreDevelopers North America this year, and some of you may have seen it. Frame the layers: evaluate to learn which tasks a small model can do; govern spend and access at a gateway; route in the data plane, which is this talk. They compose. The datapoint at the bottom is hers, from her own demo task: about 0.84 accuracy at roughly $3,500 for a large model against 0.76 at about $9 for a small one. Check these figures against her slides or recording before presenting; treat them as the shape of the trade-off.

## Slide 21

[26:00-27:00] Do not build all of it. vLLM Semantic Router is an open decision layer that picks a model per request. The Gateway API Inference Extension makes a gateway inference-aware; parts moved to llm-d repos. Agent Router, formerly Envoy AI Gateway, gives one OpenAI-compatible API over many models. READMEs were checked when I built the deck; verify before relying on them. This talk is the minimal version you can understand end to end.

## Slide 22

[27:00-27:45] Sequence matters more than dates: measure, shadow, evaluate, canary, widen. Week 1 logging only, week 2 rules in shadow mode, week 3 golden set and one low-risk canary, week 4 onward widen by task type. Nobody should flip 40% of traffic on day one.

## Slide 23

[27:45-28:45] Five takeaways. Routing belongs in the delivery plane, not every app. Savings track the share you can safely divert, so measure it first. Envoy ext_proc plus Gateway API gives portable L7 routing. Default to the strong model and earn each downgrade with evals. Start with rules, add learned classifiers when logs justify them.

## Slide 24

[28:45-30:00] Thank the audience and point to the references: Envoy, Gateway API, vLLM, Ollama, the landscape projects, and the three sources credited here: ByteByteGo, Spacestem and V. Semaan. Remind them that the figures are illustrative or reported, and the YAML is a reference blueprint to verify against current versions. Take questions. Likely questions: how accurate must the classifier be, why not just use a small model everywhere, and how do I handle streaming and tool calls.
