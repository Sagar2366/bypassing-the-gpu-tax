// Prompt classifier for Envoy ext_proc. Reads an OpenAI-style chat request,
// decides slm or llm, sets x-route-tier, and rewrites "model" to match the tier.
// Default is llm: anything unclear, risky, long, or tool-using goes to the big pool.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Decision struct {
	Tier       string  `json:"tier"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

type chatReq struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Tools    []interface{}            `json:"tools"`
	Stream   bool                     `json:"stream"`
}

var (
	slmModel  = env("SLM_MODEL", "llama3.2:1b")
	llmModel  = env("LLM_MODEL", "frontier-large")
	threshold = envF("CONFIDENCE_THRESHOLD", 0.80)
	maxChars  = int(envF("SLM_MAX_CHARS", 2000))

	// Trusted rules, not model output. Order matters: risk and injection first.
	injectionRe = regexp.MustCompile(`(?i)(ignore (all |your |previous )?(routing|instructions|rules)|classify (this )?as (easy|simple|slm)|x-route-tier|route (this )?to (the )?(slm|small))`)
	riskRe      = regexp.MustCompile(`(?i)\b(legal|lawsuit|contract|medical|diagnos|dosage|incident response|breach|vulnerab|exploit|password|credential|financial advice|tax)\b`)
	slmRe       = regexp.MustCompile(`(?i)^\s*(classify|categori[sz]e|extract|summari[sz]e|rewrite|rephrase|translate|format|fix (the )?(grammar|typos?)|convert|label|tag|detect (the )?(language|sentiment))\b`)
	hardRe      = regexp.MustCompile(`(?i)\b(plan|design|architect|debug|investigate|root cause|prove|compare .* and|why does|multi-step|trade-?offs?)\b`)
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envF(k string, d float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return d
}

func lastUserText(r *chatReq) string {
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i]["role"] == "user" {
			if s, ok := r.Messages[i]["content"].(string); ok {
				return s
			}
		}
	}
	return ""
}

func classify(r *chatReq) Decision {
	text := lastUserText(r)
	total := 0
	for _, m := range r.Messages {
		if s, ok := m["content"].(string); ok {
			total += len(s)
		}
	}
	switch {
	case text == "":
		return Decision{"llm", "no-user-text", 0.0}
	case injectionRe.MatchString(text):
		return Decision{"llm", "routing-injection-suspected", 1.0}
	case len(r.Tools) > 0:
		return Decision{"llm", "tools-present", 1.0}
	case r.Stream && false:
		return Decision{"llm", "streaming", 1.0}
	case riskRe.MatchString(text):
		return Decision{"llm", "high-risk-topic", 1.0}
	case total > maxChars:
		return Decision{"llm", "long-context", 0.95}
	case hardRe.MatchString(text):
		return Decision{"llm", "hard-task-keywords", 0.9}
	case slmRe.MatchString(text):
		return Decision{"slm", "bounded-task", 0.92}
	}
	return Decision{"llm", "unknown-intent", 0.5}
}

// apply the confidence threshold: low confidence always means llm.
func finalTier(d Decision) Decision {
	if d.Tier == "slm" && d.Confidence < threshold {
		d.Tier, d.Reason = "llm", d.Reason+"+below-threshold"
	}
	return d
}

func setHdr(k, v string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: k, RawValue: []byte(v)},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	}
}

type server struct {
	extprocv3.UnimplementedExternalProcessorServer
}

func (s *server) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	var last Decision // decision for this request, reused on the response path
	for {
		req, err := stream.Recv()
		if err == io.EOF || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			return err
		}
		var resp *extprocv3.ProcessingResponse
		switch v := req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			// Never trust client-supplied routing headers: strip before routing.
			resp = &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{Response: &extprocv3.CommonResponse{
					HeaderMutation: &extprocv3.HeaderMutation{RemoveHeaders: []string{"x-route-tier", "x-route-reason", "x-route-confidence"}},
				}}}}
		case *extprocv3.ProcessingRequest_RequestBody:
			resp, last = handleBody(v.RequestBody.GetBody())
		case *extprocv3.ProcessingRequest_ResponseHeaders:
			// Expose the decision to the client so the demo (and your dashboards) can see it.
			resp = &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{Response: &extprocv3.CommonResponse{
					HeaderMutation: &extprocv3.HeaderMutation{SetHeaders: []*corev3.HeaderValueOption{
						setHdr("x-route-tier", last.Tier), setHdr("x-route-reason", last.Reason),
						setHdr("x-route-confidence", fmt.Sprintf("%.2f", last.Confidence)),
					}}}}}}
		case *extprocv3.ProcessingRequest_ResponseBody:
			resp = &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ResponseBody{ResponseBody: &extprocv3.BodyResponse{}}}
		default:
			continue
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func handleBody(body []byte) (*extprocv3.ProcessingResponse, Decision) {
	var cr chatReq
	d := Decision{"llm", "unparseable-body", 0}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		_ = json.Unmarshal(body, &cr)
		d = finalTier(classify(&cr))
	}
	model := llmModel
	if d.Tier == "slm" {
		model = slmModel
	}
	hm := &extprocv3.HeaderMutation{SetHeaders: []*corev3.HeaderValueOption{
		setHdr("x-route-tier", d.Tier),
		setHdr("x-route-reason", d.Reason),
		setHdr("x-route-confidence", fmt.Sprintf("%.2f", d.Confidence)),
	}}
	common := &extprocv3.CommonResponse{HeaderMutation: hm, ClearRouteCache: true}
	if raw != nil {
		raw["model"] = model
		if nb, err := json.Marshal(raw); err == nil {
			common.BodyMutation = &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_Body{Body: nb}}
			hm.SetHeaders = append(hm.SetHeaders, setHdr("content-length", strconv.Itoa(len(nb))))
		}
	}
	prompt := strings.ReplaceAll(lastUserText(&cr), "\n", " ")
	if len(prompt) > 70 {
		prompt = prompt[:70] + "..."
	}
	log.Printf(`{"tier":%q,"reason":%q,"confidence":%.2f,"model":%q,"prompt":%q}`, d.Tier, d.Reason, d.Confidence, model, prompt)
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_RequestBody{
		RequestBody: &extprocv3.BodyResponse{Response: common}}}, d
}

func main() {
	addr := env("LISTEN", ":9002")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	g := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(g, &server{})
	log.Printf("classifier listening on %s (slm=%s llm=%s threshold=%.2f)", addr, slmModel, llmModel, threshold)
	log.Fatal(g.Serve(lis))
}
