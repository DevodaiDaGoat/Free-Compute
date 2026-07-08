package images

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type ImageStatus string

const (
	ImageAvailable  ImageStatus = "available"
	ImageCreating   ImageStatus = "creating"
	ImageFailed     ImageStatus = "failed"
	ImageDeleted    ImageStatus = "deleted"
)

type Image struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	UserID      string      `json:"userId"`
	OS          string      `json:"os"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	SizeGB      int         `json:"sizeGb"`
	Status      ImageStatus `json:"status"`
	IsPublic    bool        `json:"isPublic"`
	SourceVM    string      `json:"sourceVm,omitempty"`
	Checksum    string      `json:"checksum,omitempty"`
	Format      string      `json:"format"`
	MinCPU      int         `json:"minCpu"`
	MinRAMGB    int         `json:"minRamGb"`
	MinStorageGB int        `json:"minStorageGb"`
	Tags        []string    `json:"tags"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	VMID        string    `json:"vmId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SizeGB      int       `json:"sizeGb"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Manager struct {
	mu        sync.RWMutex
	images    map[string]*Image
	snapshots map[string]*Snapshot
	logger    *log.Logger
	nextID    int
}

func NewManager(logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.Default()
	}
	m := &Manager{
		images:    make(map[string]*Image),
		snapshots: make(map[string]*Snapshot),
		logger:    logger,
		nextID:    1,
	}
	m.seedDefaults()
	return m
}

func (m *Manager) seedDefaults() {
	defaults := []*Image{
		{
			ID: "img_ubuntu_2204", Name: "Ubuntu 22.04 LTS", OS: "ubuntu",
			Version: "22.04", Description: "Ubuntu 22.04 LTS (Jammy Jellyfish)", SizeGB: 10,
			Status: ImageAvailable, IsPublic: true, Format: "qcow2",
			MinCPU: 1, MinRAMGB: 2, MinStorageGB: 20,
			Tags: []string{"linux", "ubuntu", "lts"},
		},
		{
			ID: "img_ubuntu_2404", Name: "Ubuntu 24.04 LTS", OS: "ubuntu",
			Version: "24.04", Description: "Ubuntu 24.04 LTS (Noble Numbat)", SizeGB: 10,
			Status: ImageAvailable, IsPublic: true, Format: "qcow2",
			MinCPU: 1, MinRAMGB: 2, MinStorageGB: 20,
			Tags: []string{"linux", "ubuntu", "lts"},
		},
		{
			ID: "img_debian_12", Name: "Debian 12 Bookworm", OS: "debian",
			Version: "12", Description: "Debian 12 Bookworm", SizeGB: 8,
			Status: ImageAvailable, IsPublic: true, Format: "qcow2",
			MinCPU: 1, MinRAMGB: 1, MinStorageGB: 16,
			Tags: []string{"linux", "debian", "stable"},
		},
		{
			ID: "img_windows_11", Name: "Windows 11 Pro", OS: "windows",
			Version: "11", Description: "Windows 11 Pro (evaluation)", SizeGB: 32,
			Status: ImageAvailable, IsPublic: true, Format: "qcow2",
			MinCPU: 2, MinRAMGB: 4, MinStorageGB: 64,
			Tags: []string{"windows", "desktop"},
		},
	}

	now := time.Now()
	for _, img := range defaults {
		img.CreatedAt = now
		img.UpdatedAt = now
		m.images[img.ID] = img
	}
}

func (m *Manager) Create(image *Image) *Image {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("img_%d_%x", m.nextID, time.Now().UnixNano())
	m.nextID++

	image.ID = id
	image.CreatedAt = time.Now()
	image.UpdatedAt = time.Now()
	if image.Status == "" {
		image.Status = ImageAvailable
	}
	if image.Format == "" {
		image.Format = "qcow2"
	}

	m.images[id] = image
	m.logger.Printf("image created: %s (%s %s)", id[:12], image.OS, image.Version)
	return image
}

func (m *Manager) List(userID string, includePublic bool) []*Image {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Image
	for _, img := range m.images {
		if img.Status == ImageDeleted {
			continue
		}
		if img.UserID == userID || (includePublic && img.IsPublic) {
			result = append(result, img)
		}
	}
	return result
}

func (m *Manager) Get(id string) *Image {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.images[id]
}

func (m *Manager) Delete(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	img, ok := m.images[id]
	if !ok {
		return fmt.Errorf("image not found")
	}
	if img.UserID != userID && userID != "admin" {
		return fmt.Errorf("not authorized")
	}

	img.Status = ImageDeleted
	img.UpdatedAt = time.Now()
	m.logger.Printf("image deleted: %s", id[:12])
	return nil
}

func (m *Manager) CreateSnapshot(vmID, name, description string) *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("snap_%d_%x", m.nextID, time.Now().UnixNano())
	m.nextID++

	snap := &Snapshot{
		ID:          id,
		VMID:        vmID,
		Name:        name,
		Description: description,
		Status:      "completed",
		CreatedAt:   time.Now(),
	}

	m.snapshots[id] = snap
	m.logger.Printf("snapshot created: %s for VM %s", id[:12], vmID[:8])
	return snap
}

func (m *Manager) ListSnapshots(vmID string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	for _, s := range m.snapshots {
		if s.VMID == vmID {
			result = append(result, s)
		}
	}
	return result
}

func (m *Manager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.snapshots[id]; !ok {
		return fmt.Errorf("snapshot not found")
	}
	delete(m.snapshots, id)
	return nil
}

func (m *Manager) HandleImages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		m.handleList(w, r)
	case "POST":
		m.handleCreate(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleImageOps(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/images/")
	if id == "" {
		http.Error(w, `{"error":"image id required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		img := m.Get(id)
		if img == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not found"})
			return
		}
		writeJSON(w, http.StatusOK, img)
	case "DELETE":
		userID := r.URL.Query().Get("userId")
		if err := m.Delete(id, userID); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		m.handleCreateSnapshot(w, r)
	case "GET":
		vmID := r.URL.Query().Get("vmId")
		if vmID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vmId required"})
			return
		}
		snaps := m.ListSnapshots(vmID)
		writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) HandleSnapshotOps(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/snapshots/")
	if id == "" {
		http.Error(w, `{"error":"snapshot id required"}`, http.StatusBadRequest)
		return
	}

	if r.Method == "DELETE" {
		if err := m.DeleteSnapshot(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	} else {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleList(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	includePublic := r.URL.Query().Get("public") != "false"

	images := m.List(userID, includePublic)
	writeJSON(w, http.StatusOK, map[string]any{
		"images": images,
		"count":  len(images),
	})
}

func (m *Manager) handleCreate(w http.ResponseWriter, r *http.Request) {
	var img Image
	if err := json.NewDecoder(r.Body).Decode(&img); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if img.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	result := m.Create(&img)
	writeJSON(w, http.StatusCreated, result)
}

func (m *Manager) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VMID        string `json:"vmId"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.VMID == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vmId and name required"})
		return
	}

	snap := m.CreateSnapshot(req.VMID, req.Name, req.Description)
	writeJSON(w, http.StatusCreated, snap)
}

func extractID(path, prefix string) string {
	idx := len(prefix)
	if idx >= len(path) {
		return ""
	}
	id := path[idx:]
	if idx := indexByte(id, '/'); idx >= 0 {
		id = id[:idx]
	}
	return id
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
