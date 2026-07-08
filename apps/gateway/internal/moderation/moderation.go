package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type ReportInput struct {
	ReportID   string            `json:"reportId"`
	ReporterID string            `json:"reporterId"`
	TargetType string            `json:"targetType"`
	TargetID   string            `json:"targetId"`
	Reason     string            `json:"reason"`
	Evidence   map[string]string `json:"evidence"`
}

type Action struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type ModerationAI interface {
	Triage(ctx context.Context, in ReportInput) (Action, error)
}

type HeuristicModerator struct{}

func (m *HeuristicModerator) Triage(ctx context.Context, in ReportInput) (Action, error) {
	reason := strings.ToLower(in.Reason)
	switch {
	case strings.Contains(reason, "csam"):
		return Action{Decision: "ban", Reason: "severe policy violation: csam"}, nil
	case strings.Contains(reason, "illegal"):
		return Action{Decision: "ban", Reason: "illegal activity reported"}, nil
	case strings.Contains(reason, "abuse"),
		strings.Contains(reason, "mining"),
		strings.Contains(reason, "attack"):
		return Action{Decision: "review", Reason: "report flagged for human review"}, nil
	default:
		return Action{Decision: "ignore", Reason: "no policy match"}, nil
	}
}

type LLMModerator struct {
	URL string
	Key string
}

func NewLLMModerator(url, key string) *LLMModerator {
	return &LLMModerator{URL: url, Key: key}
}

func NewModerationAIFromEnv() ModerationAI {
	if url := os.Getenv("FREECOMPUTE_MODERATION_LLM_URL"); url != "" {
		return NewLLMModerator(url, os.Getenv("FREECOMPUTE_MODERATION_LLM_KEY"))
	}
	return &HeuristicModerator{}
}

func (m *LLMModerator) Triage(ctx context.Context, in ReportInput) (Action, error) {
	if m.URL == "" {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}

	payload, err := json.Marshal(map[string]interface{}{"report": in})
	if err != nil {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(payload))
	if err != nil {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}
	req.Header.Set("Authorization", "Bearer "+m.Key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}

	var action Action
	if err := json.Unmarshal(body, &action); err != nil {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}
	if action.Decision == "" {
		return (&HeuristicModerator{}).Triage(ctx, in)
	}

	return action, nil
}
