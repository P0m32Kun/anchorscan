package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
)

const commandGateTTL = 5 * time.Minute

type commandGateView struct {
	Level            string   `json:"level"`
	ReviewStatus     string   `json:"review_status"`
	SafetyMode       string   `json:"safety_mode"`
	Effects          []string `json:"effects"`
	Cleanup          string   `json:"cleanup"`
	Message          string   `json:"message"`
	AcknowledgeLabel string   `json:"acknowledge_label"`
	Challenge        string   `json:"challenge,omitempty"`
}

type commandGateRequest struct {
	Action      string
	Tool        string
	Key         string
	Fingerprint string
}

type commandChallenge struct {
	Key       string
	ExpiresAt time.Time
}

type toolPrefill struct {
	Tool           string
	RawArgs        string
	ProjectID      string
	ZoneID         string
	VerificationID string
	ReturnURL      string
	ExpiresAt      time.Time
}

type commandGateStore struct {
	mu         sync.Mutex
	challenges map[string]commandChallenge
	prefills   map[string]toolPrefill
}

func newCommandGateStore() *commandGateStore {
	return &commandGateStore{challenges: map[string]commandChallenge{}, prefills: map[string]toolPrefill{}}
}

func newOneTimeToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (g *commandGateStore) issueChallenge(key string, now time.Time) (string, error) {
	token, err := newOneTimeToken()
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeExpiredLocked(now)
	g.challenges[token] = commandChallenge{Key: key, ExpiresAt: now.Add(commandGateTTL)}
	return token, nil
}

func (g *commandGateStore) consumeChallenge(token, key string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	challenge, ok := g.challenges[token]
	delete(g.challenges, token)
	if !ok || now.After(challenge.ExpiresAt) {
		return false
	}
	return challenge.Key == key
}

func (g *commandGateStore) issuePrefill(value toolPrefill, now time.Time) (string, error) {
	token, err := newOneTimeToken()
	if err != nil {
		return "", err
	}
	value.ExpiresAt = now.Add(commandGateTTL)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeExpiredLocked(now)
	g.prefills[token] = value
	return token, nil
}

func (g *commandGateStore) consumePrefill(token, tool string, now time.Time) (toolPrefill, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	value, ok := g.prefills[token]
	delete(g.prefills, token)
	if !ok || now.After(value.ExpiresAt) || value.Tool != tool {
		return toolPrefill{}, false
	}
	return value, true
}

func (g *commandGateStore) removeExpiredLocked(now time.Time) {
	for token, challenge := range g.challenges {
		if now.After(challenge.ExpiresAt) {
			delete(g.challenges, token)
		}
	}
	for token, prefill := range g.prefills {
		if now.After(prefill.ExpiresAt) {
			delete(g.prefills, token)
		}
	}
}

func commandGateKey(req commandGateRequest, entry knowledgebase.Entry) string {
	return strings.Join([]string{req.Action, req.Tool, req.Key, req.Fingerprint, entry.ID}, "\x00")
}

func commandForTool(entry knowledgebase.Entry, tool string) string {
	switch tool {
	case "nuclei":
		return entry.Commands.Nuclei
	case "nmap":
		return entry.Commands.NmapNSE
	case "msf":
		return entry.Commands.Metasploit
	default:
		return ""
	}
}

func commandGateViewForEntry(entry knowledgebase.Entry, tool string, diagnostics []knowledgebase.Diagnostic) (commandGateView, error) {
	if strings.TrimSpace(commandForTool(entry, tool)) == "" {
		return commandGateView{}, fmt.Errorf("知识库未提供可用命令")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.EntryID == entry.ID && strings.Contains(diagnostic.Reason, "命令无效") {
			return commandGateView{}, fmt.Errorf("知识库命令格式无效")
		}
	}
	if entry.ReviewStatus != knowledgebase.ReviewStatusStable && entry.ReviewStatus != knowledgebase.ReviewStatusNeedsReview && entry.ReviewStatus != knowledgebase.ReviewStatusLegacyUnknown {
		return commandGateView{}, fmt.Errorf("知识库复核状态无效")
	}
	if !validCommandGateSafety(entry) {
		return commandGateView{}, fmt.Errorf("知识库 safety 无效")
	}
	if entry.ReviewStatus == knowledgebase.ReviewStatusLegacyUnknown && entry.Safety.Mode != knowledgebase.SafetyLegacyUnknown {
		return commandGateView{}, fmt.Errorf("知识库 legacy safety 无效")
	}
	if entry.Safety.Mode == knowledgebase.SafetyLegacyUnknown && entry.ReviewStatus != knowledgebase.ReviewStatusLegacyUnknown {
		return commandGateView{}, fmt.Errorf("知识库 legacy review status 无效")
	}

	view := commandGateView{
		ReviewStatus: string(entry.ReviewStatus),
		SafetyMode:   string(entry.Safety.Mode),
		Effects:      append([]string{}, entry.Safety.Effects...),
		Cleanup:      entry.Safety.Cleanup,
	}
	switch {
	case entry.Safety.Mode == knowledgebase.SafetyLegacyUnknown:
		view.Level = "legacy-unknown"
		view.Message = "旧 Markdown 未声明 safety；此命令按至少 manual-gated 强度处理。"
		view.AcknowledgeLabel = "我已知悉旧 Markdown 未声明 safety，并确认生成命令"
	case entry.Safety.Mode == knowledgebase.SafetyManualGated:
		view.Level = "manual-gated"
		view.Message = "此命令可能对目标产生主动影响；请确认下列 effects 与 cleanup。"
		view.AcknowledgeLabel = "我已查看 effects 与 cleanup，确认生成命令"
	case entry.Safety.Mode == knowledgebase.SafetyOptional:
		view.Level = "optional"
		view.Message = "此命令包含 authentication-attempt；请确认明确的授权范围。"
		view.AcknowledgeLabel = "我已确认授权范围，生成命令"
	case entry.ReviewStatus == knowledgebase.ReviewStatusNeedsReview:
		view.Level = "needs-review"
		view.Message = "此条目仍处于待复核状态；请确认已复核来源后生成命令。"
		view.AcknowledgeLabel = "我已复核来源并确认生成命令"
	default:
		view.Level = "safe"
	}
	return view, nil
}

func validCommandGateSafety(entry knowledgebase.Entry) bool {
	mode := entry.Safety.Mode
	if mode == knowledgebase.SafetyLegacyUnknown {
		return len(entry.Safety.Effects) == 0 && strings.TrimSpace(entry.Safety.Cleanup) == ""
	}
	if mode != knowledgebase.SafetySafe && mode != knowledgebase.SafetyOptional && mode != knowledgebase.SafetyManualGated {
		return false
	}
	seen := map[string]bool{}
	hasFileEffect := false
	for _, effect := range entry.Safety.Effects {
		if seen[effect] {
			return false
		}
		seen[effect] = true
		switch effect {
		case "authentication-attempt", "file-read", "test-file-create", "test-file-delete", "controlled-command", "oast":
		default:
			return false
		}
		hasFileEffect = hasFileEffect || effect == "test-file-create" || effect == "test-file-delete"
	}
	if mode == knowledgebase.SafetySafe && len(entry.Safety.Effects) != 0 {
		return false
	}
	if mode == knowledgebase.SafetyOptional && (len(entry.Safety.Effects) != 1 || entry.Safety.Effects[0] != "authentication-attempt") {
		return false
	}
	if mode == knowledgebase.SafetyManualGated && len(entry.Safety.Effects) == 0 {
		return false
	}
	return !hasFileEffect || strings.TrimSpace(entry.Safety.Cleanup) != ""
}

func commandGateRequired(view commandGateView) bool {
	return view.Level != "safe"
}

func (s *server) enforceCommandGate(w http.ResponseWriter, r *http.Request, req commandGateRequest, entry knowledgebase.Entry) bool {
	view, err := commandGateViewForEntry(entry, req.Tool, s.catalog.Diagnostics())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return false
	}
	if !commandGateRequired(view) {
		return true
	}
	key := commandGateKey(req, entry)
	token := strings.TrimSpace(r.FormValue("gate_token"))
	if token == "" {
		token, err = s.commandGate.issueChallenge(key, s.opts.Now())
		if err != nil {
			http.Error(w, "无法建立一次性确认挑战", http.StatusInternalServerError)
			return false
		}
		view.Challenge = token
		writeJSON(w, http.StatusPreconditionRequired, map[string]any{"error": "需要命令确认", "gate": view})
		return false
	}
	if r.FormValue("acknowledge") != "1" || !s.commandGate.consumeChallenge(token, key, s.opts.Now()) {
		http.Error(w, "确认挑战无效或已消费", http.StatusForbidden)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *server) toolLink(tool string, value toolPrefill) (string, error) {
	token, err := s.commandGate.issuePrefill(value, s.opts.Now())
	if err != nil {
		return "", err
	}
	return "/tools/" + tool + "?gate_token=" + token, nil
}

func (s *server) toolPrefill(token, tool string) (toolPrefill, bool) {
	return s.commandGate.consumePrefill(token, tool, s.opts.Now())
}
