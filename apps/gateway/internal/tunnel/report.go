package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
	"github.com/freecompute/free-compute/apps/gateway/internal/database"
	"github.com/freecompute/free-compute/apps/gateway/internal/moderation"
)

type ReportHandler struct {
	db       *database.DB
	auth     *auth.AuthManager
	ai       moderation.ModerationAI
	aiActive func() bool
}

func NewReportHandler(db *database.DB, authMgr *auth.AuthManager, ai moderation.ModerationAI, aiActive func() bool) *ReportHandler {
	if aiActive == nil {
		aiActive = func() bool { return false }
	}
	return &ReportHandler{
		db:       db,
		auth:     authMgr,
		ai:       ai,
		aiActive: aiActive,
	}
}

type createReportRequest struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Reason     string `json:"reason"`
}

func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	user := auth.UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if h.db == nil {
		http.Error(w, `{"error":"reports unavailable"}`, http.StatusInternalServerError)
		return
	}
	var req createReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.TargetType == "" || req.TargetID == "" {
		http.Error(w, `{"error":"targetType and targetId required"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	row := database.ReportsRow{
		ReporterID: user.ID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Reason:     req.Reason,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.db.CreateReport(&row); err != nil {
		http.Error(w, `{"error":"failed to create report"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": row.ID, "status": "pending"})
}

func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if h.db == nil {
		http.Error(w, `{"error":"reports unavailable"}`, http.StatusInternalServerError)
		return
	}
	reports, err := h.db.ListReports()
	if err != nil {
		http.Error(w, `{"error":"failed to list reports"}`, http.StatusInternalServerError)
		return
	}
	if reports == nil {
		reports = []*database.ReportsRow{}
	}
	writeJSON(w, http.StatusOK, reports)
}

var reportActionStatus = map[string]string{
	"warn":    "warned",
	"ban":     "banned",
	"pause":   "paused",
	"resolve": "resolved",
	"ignore":  "ignored",
}

func (h *ReportHandler) Action(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	user := auth.UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if h.db == nil {
		http.Error(w, `{"error":"reports unavailable"}`, http.StatusInternalServerError)
		return
	}
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.Action == "" {
		http.Error(w, `{"error":"id and action required"}`, http.StatusBadRequest)
		return
	}

	status, ok := reportActionStatus[req.Action]
	if !ok {
		http.Error(w, `{"error":"invalid action"}`, http.StatusBadRequest)
		return
	}

	decision := req.Action
	if h.aiActive != nil && h.aiActive() && h.ai != nil {
		suggestion, err := h.ai.Triage(r.Context(), moderation.ReportInput{
			ReportID:   req.ID,
			ReporterID: user.ID,
			TargetType: "report",
			TargetID:   req.ID,
			Reason:     "moderator action requested: " + req.Action,
		})
		if err == nil && suggestion.Decision != "" && suggestion.Decision != "ignore" {
			decision = suggestion.Decision
		}
	}

	if err := h.db.UpdateReport(req.ID, user.ID, status, decision); err != nil {
		http.Error(w, `{"error":"failed to update report"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "decision": decision})
}

// ensure context import is used even if AI triage path changes
var _ = context.Background
