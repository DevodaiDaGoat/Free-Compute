package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	mu sync.RWMutex
}

type UserRow struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"`
	DisplayName    string    `json:"displayName"`
	AvatarURL      string    `json:"avatarUrl"`
	TailscaleIP    string    `json:"tailscaleIp"`
	TailscaleCIDR  string    `json:"tailscaleCidr"`
	TailscaleKey   string    `json:"-"`
	StorageUsed    int64     `json:"storageUsed"`
	StorageQuota   int64     `json:"storageQuota"`
	Credits        int64     `json:"credits"`
	Role           string    `json:"role"`
	Verified       bool      `json:"verified"`
	Banned         bool      `json:"banned"`
	Preferences    string    `json:"preferences"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type ReportsRow struct {
	ID          string    `json:"id"`
	ReporterID  string    `json:"reporterId"`
	TargetType  string    `json:"targetType"`
	TargetID    string    `json:"targetId"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`
	ReviewerID  string    `json:"reviewerId"`
	Action      string    `json:"action"`
	CreatedAt   string    `json:"createdAt"`
	UpdatedAt   string    `json:"updatedAt"`
}

type SyncRequestRow struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Status    string    `json:"status"`
	ReviewerID string   `json:"reviewerId"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
}

type SessionRow struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	VMMac          string    `json:"vmMac"`
	Type           string    `json:"type"`
	Mode           string    `json:"mode"`
	Status         string    `json:"status"`
	Protocol       string    `json:"protocol"`
	HostAddr       string    `json:"hostAddr"`
	StreamPreset   string    `json:"streamPreset"`
	ResourceClass  string    `json:"resourceClass"`
	Permissions    string    `json:"permissions"`
	Approved       bool      `json:"approved"`
	ApprovedBy     string    `json:"approvedBy"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	EndedAt        *time.Time `json:"endedAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type VMRow struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	HostID         string    `json:"hostId"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	VCPUs          int       `json:"vcpus"`
	MemoryMB       int       `json:"memoryMb"`
	DiskMB         int64     `json:"diskMb"`
	OSType         string    `json:"osType"`
	ImageURL       string    `json:"imageUrl"`
	MacAddr        string    `json:"macAddr"`
	SSHPort        int       `json:"sshPort"`
	VNCPort        int       `json:"vncPort"`
	PausedAt       *time.Time `json:"pausedAt"`
	TerminatedAt   *time.Time `json:"terminatedAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type HostRow struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Addr           string    `json:"addr"`
	Region         string    `json:"region"`
	Status         string    `json:"status"`
	TotalCPU       int       `json:"totalCpu"`
	UsedCPU        int       `json:"usedCpu"`
	TotalMemoryMB  int64     `json:"totalMemoryMb"`
	UsedMemoryMB   int64     `json:"usedMemoryMb"`
	TotalDiskMB    int64     `json:"totalDiskMb"`
	UsedDiskMB     int64     `json:"usedDiskMb"`
	GPUModel       string    `json:"gpuModel"`
	GPUVRAMMB      int64     `json:"gpuVramMb"`
	LoadAvg1       float64   `json:"loadAvg1"`
	TailscaleIP    string    `json:"tailscaleIp"`
	LastHeartbeat  time.Time `json:"lastHeartbeat"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ThreatRow struct {
	ID          string    `json:"id"`
	VMID        string    `json:"vmId"`
	UserID      string    `json:"userId"`
	Type        string    `json:"type"`
	Level       string    `json:"level"`
	Description string    `json:"description"`
	Evidence    string    `json:"evidence"`
	Screenshot  string    `json:"screenshot"`
	Action      string    `json:"action"`
	Resolved    bool      `json:"resolved"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type QueueRow struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	VMID        string    `json:"vmId"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	Region      string    `json:"region"`
	ResourceClass string  `json:"resourceClass"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AuditLogRow struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	UserID    string    `json:"userId"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	IPAddr    string    `json:"ipAddr"`
	CreatedAt time.Time `json:"createdAt"`
}

func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=mmap_size(268435456)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)

	db := &DB{DB: sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	_ = db.Exec("PRAGMA cache_size = -64000")
	_ = db.Exec("PRAGMA mmap_size = 268435456")
	_ = db.Exec("PRAGMA synchronous = NORMAL")
	_ = db.Exec("PRAGMA journal_size_limit = 67108864")

	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		// Best-effort; ignore errors on older SQLite versions
		_ = err
	}

	return db, nil
}

func (db *DB) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			tailscale_ip TEXT NOT NULL DEFAULT '',
			tailscale_cidr TEXT NOT NULL DEFAULT '',
			tailscale_key TEXT NOT NULL DEFAULT '',
			storage_used INTEGER NOT NULL DEFAULT 0,
			storage_quota INTEGER NOT NULL DEFAULT 107374182400,
			credits INTEGER NOT NULL DEFAULT 0,
			role TEXT NOT NULL DEFAULT 'user',
			verified INTEGER NOT NULL DEFAULT 1,
			banned INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			vm_mac TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'desktop',
			mode TEXT NOT NULL DEFAULT 'desktop',
			status TEXT NOT NULL DEFAULT 'pending',
			protocol TEXT NOT NULL DEFAULT 'webrtc',
			host_addr TEXT NOT NULL DEFAULT '',
			stream_preset TEXT NOT NULL DEFAULT 'safe',
			resource_class TEXT NOT NULL DEFAULT 'basic',
			permissions TEXT NOT NULL DEFAULT '{}',
			approved INTEGER NOT NULL DEFAULT 0,
			approved_by TEXT NOT NULL DEFAULT '',
			expires_at TEXT,
			ended_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS vms (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			host_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'stopped',
			vcpus INTEGER NOT NULL DEFAULT 2,
			memory_mb INTEGER NOT NULL DEFAULT 4096,
			disk_mb INTEGER NOT NULL DEFAULT 51200,
			os_type TEXT NOT NULL DEFAULT 'linux',
			image_url TEXT NOT NULL DEFAULT '',
			mac_addr TEXT NOT NULL DEFAULT '',
			ssh_port INTEGER NOT NULL DEFAULT 0,
			vnc_port INTEGER NOT NULL DEFAULT 0,
			paused_at TEXT,
			terminated_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (host_id) REFERENCES hosts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS hosts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			addr TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT 'us-east',
			status TEXT NOT NULL DEFAULT 'offline',
			total_cpu INTEGER NOT NULL DEFAULT 0,
			used_cpu INTEGER NOT NULL DEFAULT 0,
			total_memory_mb INTEGER NOT NULL DEFAULT 0,
			used_memory_mb INTEGER NOT NULL DEFAULT 0,
			total_disk_mb INTEGER NOT NULL DEFAULT 0,
			used_disk_mb INTEGER NOT NULL DEFAULT 0,
			gpu_model TEXT NOT NULL DEFAULT '',
			gpu_vram_mb INTEGER NOT NULL DEFAULT 0,
			load_avg1 REAL NOT NULL DEFAULT 0,
			tailscale_ip TEXT NOT NULL DEFAULT '',
			last_heartbeat TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS threats (
			id TEXT PRIMARY KEY,
			vm_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			level TEXT NOT NULL DEFAULT 'low',
			description TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '{}',
			screenshot TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			resolved INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (vm_id) REFERENCES vms(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS queue (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			vm_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'waiting',
			priority INTEGER NOT NULL DEFAULT 0,
			region TEXT NOT NULL DEFAULT 'us-east',
			resource_class TEXT NOT NULL DEFAULT 'basic',
			position INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (vm_id) REFERENCES vms(id)
		)`,
		`CREATE TABLE IF NOT EXISTS storage_files (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			mime_type TEXT NOT NULL DEFAULT '',
			file_size INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			details TEXT NOT NULL DEFAULT '{}',
			ip_addr TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:60], err)
		}
	}

	migrations := []string{
		`ALTER TABLE users ADD COLUMN preferences TEXT NOT NULL DEFAULT '{}'`,

		`CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			reporter_id TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			reviewer_id TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (reporter_id) REFERENCES users(id)
		)`,

		`CREATE TABLE IF NOT EXISTS personalization_sync_requests (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reviewer_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_sessions_user       ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_vms_user           ON vms(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_threats_user       ON threats(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_threats_vm         ON threats(vm_id)`,
		`CREATE INDEX IF NOT EXISTS idx_queue_user_status  ON queue(user_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_storage_files_user ON storage_files(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_user         ON audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_session      ON audit_logs(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reports_target     ON reports(target_id, status)`,
	}

	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
				continue
			}
			return fmt.Errorf("migrate %q: %w", stmt[:60], err)
		}
	}

	return nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// --- Users ---

func (db *DB) CreateUser(u *UserRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO users (id, email, password_hash, display_name, avatar_url,
		tailscale_ip, tailscale_cidr, tailscale_key, storage_used, storage_quota, credits, role,
		verified, banned, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.AvatarURL,
		u.TailscaleIP, u.TailscaleCIDR, u.TailscaleKey, u.StorageUsed, u.StorageQuota,
		u.Credits, u.Role, boolToInt(u.Verified), boolToInt(u.Banned),
		u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetUserByID(id string) (*UserRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, email, password_hash, display_name, avatar_url,
		tailscale_ip, tailscale_cidr, tailscale_key, storage_used, storage_quota, credits, role,
		verified, banned, preferences, created_at, updated_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (db *DB) GetUserByEmail(email string) (*UserRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, email, password_hash, display_name, avatar_url,
		tailscale_ip, tailscale_cidr, tailscale_key, storage_used, storage_quota, credits, role,
		verified, banned, preferences, created_at, updated_at FROM users WHERE email=?`, email)
	return scanUser(row)
}

func (db *DB) UpdateUser(u *UserRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	u.UpdatedAt = time.Now()
	_, err := db.Exec(`UPDATE users SET email=?, password_hash=?, display_name=?, avatar_url=?,
		tailscale_ip=?, tailscale_cidr=?, tailscale_key=?, storage_used=?, storage_quota=?, credits=?,
		role=?, verified=?, banned=?, updated_at=? WHERE id=?`,
		u.Email, u.PasswordHash, u.DisplayName, u.AvatarURL,
		u.TailscaleIP, u.TailscaleCIDR, u.TailscaleKey, u.StorageUsed, u.StorageQuota,
		u.Credits, u.Role, boolToInt(u.Verified), boolToInt(u.Banned),
		u.UpdatedAt.Format(time.RFC3339), u.ID)
	return err
}

func (db *DB) SetUserRole(userID, role string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`UPDATE users SET role=?, updated_at=? WHERE id=?`,
		role, time.Now().UTC().Format(time.RFC3339), userID)
	return err
}

func (db *DB) ListUsers() ([]*UserRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, email, password_hash, display_name, avatar_url,
		tailscale_ip, tailscale_cidr, tailscale_key, storage_used, storage_quota, credits, role,
		verified, banned, preferences, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*UserRow
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) DeleteUser(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (db *DB) CountUsers() (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func scanUser(s interface {
	Scan(dest ...interface{}) error
}) (*UserRow, error) {
	var (
		u              UserRow
		createdAt, updatedAt string
		verified, banned    int
	)
	err := s.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.AvatarURL,
		&u.TailscaleIP, &u.TailscaleCIDR, &u.TailscaleKey, &u.StorageUsed, &u.StorageQuota,
		&u.Credits, &u.Role, &verified, &banned, &u.Preferences, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	u.Verified = verified != 0
	u.Banned = banned != 0
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &u, nil
}

// --- Sessions ---

func (db *DB) CreateSession(s *SessionRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	var expires, ended *string
	if s.ExpiresAt != nil {
		v := s.ExpiresAt.Format(time.RFC3339)
		expires = &v
	}
	if s.EndedAt != nil {
		v := s.EndedAt.Format(time.RFC3339)
		ended = &v
	}
	_, err := db.Exec(`INSERT INTO sessions (id, user_id, vm_mac, type, mode, status, protocol, host_addr,
		stream_preset, resource_class, permissions, approved, approved_by, expires_at, ended_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.UserID, s.VMMac, s.Type, s.Mode, s.Status, s.Protocol, s.HostAddr,
		s.StreamPreset, s.ResourceClass, s.Permissions, boolToInt(s.Approved), s.ApprovedBy,
		expires, ended, s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetSession(id string) (*SessionRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, user_id, vm_mac, type, mode, status, protocol, host_addr,
		stream_preset, resource_class, permissions, approved, approved_by, expires_at, ended_at, created_at, updated_at
		FROM sessions WHERE id=?`, id)
	return scanSession(row)
}

func (db *DB) ListSessionsByUser(userID string) ([]*SessionRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, vm_mac, type, mode, status, protocol, host_addr,
		stream_preset, resource_class, permissions, approved, approved_by, expires_at, ended_at, created_at, updated_at
		FROM sessions WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []*SessionRow
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (db *DB) ListActiveSessions() ([]*SessionRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, vm_mac, type, mode, status, protocol, host_addr,
		stream_preset, resource_class, permissions, approved, approved_by, expires_at, ended_at, created_at, updated_at
		FROM sessions WHERE status='active' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []*SessionRow
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (db *DB) UpdateSession(s *SessionRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	s.UpdatedAt = time.Now()
	var expires, ended *string
	if s.ExpiresAt != nil {
		v := s.ExpiresAt.Format(time.RFC3339)
		expires = &v
	}
	if s.EndedAt != nil {
		v := s.EndedAt.Format(time.RFC3339)
		ended = &v
	}
	_, err := db.Exec(`UPDATE sessions SET user_id=?, vm_mac=?, type=?, mode=?, status=?, protocol=?,
		host_addr=?, stream_preset=?, resource_class=?, permissions=?, approved=?, approved_by=?,
		expires_at=?, ended_at=?, updated_at=? WHERE id=?`,
		s.UserID, s.VMMac, s.Type, s.Mode, s.Status, s.Protocol, s.HostAddr,
		s.StreamPreset, s.ResourceClass, s.Permissions, boolToInt(s.Approved), s.ApprovedBy,
		expires, ended, s.UpdatedAt.Format(time.RFC3339), s.ID)
	return err
}

func scanSession(s interface {
	Scan(dest ...interface{}) error
}) (*SessionRow, error) {
	var (
		sess              SessionRow
		createdAt, updatedAt string
		approved              int
		expires, ended      *string
	)
	err := s.Scan(&sess.ID, &sess.UserID, &sess.VMMac, &sess.Type, &sess.Mode, &sess.Status,
		&sess.Protocol, &sess.HostAddr, &sess.StreamPreset, &sess.ResourceClass, &sess.Permissions,
		&approved, &sess.ApprovedBy, &expires, &ended, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	sess.Approved = approved != 0
	if expires != nil {
		t, _ := time.Parse(time.RFC3339, *expires)
		sess.ExpiresAt = &t
	}
	if ended != nil {
		t, _ := time.Parse(time.RFC3339, *ended)
		sess.EndedAt = &t
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &sess, nil
}

// --- VMs ---

func (db *DB) CreateVM(vm *VMRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	var paused, terminated *string
	if vm.PausedAt != nil {
		v := vm.PausedAt.Format(time.RFC3339)
		paused = &v
	}
	if vm.TerminatedAt != nil {
		v := vm.TerminatedAt.Format(time.RFC3339)
		terminated = &v
	}
	_, err := db.Exec(`INSERT INTO vms (id, user_id, host_id, name, status, vcpus, memory_mb, disk_mb,
		os_type, image_url, mac_addr, ssh_port, vnc_port, paused_at, terminated_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		vm.ID, vm.UserID, vm.HostID, vm.Name, vm.Status, vm.VCPUs, vm.MemoryMB, vm.DiskMB,
		vm.OSType, vm.ImageURL, vm.MacAddr, vm.SSHPort, vm.VNCPort,
		paused, terminated, vm.CreatedAt.Format(time.RFC3339), vm.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetVM(id string) (*VMRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, user_id, host_id, name, status, vcpus, memory_mb, disk_mb,
		os_type, image_url, mac_addr, ssh_port, vnc_port, paused_at, terminated_at, created_at, updated_at
		FROM vms WHERE id=?`, id)
	return scanVM(row)
}

func (db *DB) ListVMsByUser(userID string) ([]*VMRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, host_id, name, status, vcpus, memory_mb, disk_mb,
		os_type, image_url, mac_addr, ssh_port, vnc_port, paused_at, terminated_at, created_at, updated_at
		FROM vms WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vms []*VMRow
	for rows.Next() {
		vm, err := scanVM(rows)
		if err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

func (db *DB) ListVMsByHost(hostID string) ([]*VMRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, host_id, name, status, vcpus, memory_mb, disk_mb,
		os_type, image_url, mac_addr, ssh_port, vnc_port, paused_at, terminated_at, created_at, updated_at
		FROM vms WHERE host_id=? ORDER BY created_at DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vms []*VMRow
	for rows.Next() {
		vm, err := scanVM(rows)
		if err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

func (db *DB) ListActiveVMs() ([]*VMRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, host_id, name, status, vcpus, memory_mb, disk_mb,
		os_type, image_url, mac_addr, ssh_port, vnc_port, paused_at, terminated_at, created_at, updated_at
		FROM vms WHERE status NOT IN ('stopped','terminated') ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vms []*VMRow
	for rows.Next() {
		vm, err := scanVM(rows)
		if err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

func (db *DB) UpdateVM(vm *VMRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	vm.UpdatedAt = time.Now()
	var paused, terminated *string
	if vm.PausedAt != nil {
		v := vm.PausedAt.Format(time.RFC3339)
		paused = &v
	}
	if vm.TerminatedAt != nil {
		v := vm.TerminatedAt.Format(time.RFC3339)
		terminated = &v
	}
	_, err := db.Exec(`UPDATE vms SET user_id=?, host_id=?, name=?, status=?, vcpus=?, memory_mb=?, disk_mb=?,
		os_type=?, image_url=?, mac_addr=?, ssh_port=?, vnc_port=?, paused_at=?, terminated_at=?, updated_at=?
		WHERE id=?`,
		vm.UserID, vm.HostID, vm.Name, vm.Status, vm.VCPUs, vm.MemoryMB, vm.DiskMB,
		vm.OSType, vm.ImageURL, vm.MacAddr, vm.SSHPort, vm.VNCPort,
		paused, terminated, vm.UpdatedAt.Format(time.RFC3339), vm.ID)
	return err
}

func (db *DB) DeleteVM(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`DELETE FROM vms WHERE id=?`, id)
	return err
}

func scanVM(s interface {
	Scan(dest ...interface{}) error
}) (*VMRow, error) {
	var (
		vm                     VMRow
		createdAt, updatedAt     string
		paused, terminated       *string
	)
	err := s.Scan(&vm.ID, &vm.UserID, &vm.HostID, &vm.Name, &vm.Status,
		&vm.VCPUs, &vm.MemoryMB, &vm.DiskMB, &vm.OSType, &vm.ImageURL,
		&vm.MacAddr, &vm.SSHPort, &vm.VNCPort, &paused, &terminated,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if paused != nil {
		t, _ := time.Parse(time.RFC3339, *paused)
		vm.PausedAt = &t
	}
	if terminated != nil {
		t, _ := time.Parse(time.RFC3339, *terminated)
		vm.TerminatedAt = &t
	}
	vm.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	vm.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &vm, nil
}

// --- Hosts ---

func (db *DB) CreateHost(h *HostRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO hosts (id, name, addr, region, status, total_cpu, used_cpu,
		total_memory_mb, used_memory_mb, total_disk_mb, used_disk_mb, gpu_model, gpu_vram_mb,
		load_avg1, tailscale_ip, last_heartbeat, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		h.ID, h.Name, h.Addr, h.Region, h.Status, h.TotalCPU, h.UsedCPU,
		h.TotalMemoryMB, h.UsedMemoryMB, h.TotalDiskMB, h.UsedDiskMB,
		h.GPUModel, h.GPUVRAMMB, h.LoadAvg1, h.TailscaleIP,
		h.LastHeartbeat.Format(time.RFC3339), h.CreatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetHost(id string) (*HostRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, name, addr, region, status, total_cpu, used_cpu,
		total_memory_mb, used_memory_mb, total_disk_mb, used_disk_mb, gpu_model, gpu_vram_mb,
		load_avg1, tailscale_ip, last_heartbeat, created_at FROM hosts WHERE id=?`, id)
	return scanHost(row)
}

func (db *DB) ListHosts() ([]*HostRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, name, addr, region, status, total_cpu, used_cpu,
		total_memory_mb, used_memory_mb, total_disk_mb, used_disk_mb, gpu_model, gpu_vram_mb,
		load_avg1, tailscale_ip, last_heartbeat, created_at FROM hosts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []*HostRow
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (db *DB) ListAvailableHosts() ([]*HostRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, name, addr, region, status, total_cpu, used_cpu,
		total_memory_mb, used_memory_mb, total_disk_mb, used_disk_mb, gpu_model, gpu_vram_mb,
		load_avg1, tailscale_ip, last_heartbeat, created_at FROM hosts WHERE status='online' ORDER BY load_avg1 ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []*HostRow
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (db *DB) UpdateHost(h *HostRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`UPDATE hosts SET name=?, addr=?, region=?, status=?, total_cpu=?, used_cpu=?,
		total_memory_mb=?, used_memory_mb=?, total_disk_mb=?, used_disk_mb=?, gpu_model=?, gpu_vram_mb=?,
		load_avg1=?, tailscale_ip=?, last_heartbeat=? WHERE id=?`,
		h.Name, h.Addr, h.Region, h.Status, h.TotalCPU, h.UsedCPU,
		h.TotalMemoryMB, h.UsedMemoryMB, h.TotalDiskMB, h.UsedDiskMB,
		h.GPUModel, h.GPUVRAMMB, h.LoadAvg1, h.TailscaleIP,
		h.LastHeartbeat.Format(time.RFC3339), h.ID)
	return err
}

func scanHost(s interface {
	Scan(dest ...interface{}) error
}) (*HostRow, error) {
	var (
		h                      HostRow
		lastHb, createdAt      string
	)
	err := s.Scan(&h.ID, &h.Name, &h.Addr, &h.Region, &h.Status,
		&h.TotalCPU, &h.UsedCPU, &h.TotalMemoryMB, &h.UsedMemoryMB,
		&h.TotalDiskMB, &h.UsedDiskMB, &h.GPUModel, &h.GPUVRAMMB,
		&h.LoadAvg1, &h.TailscaleIP, &lastHb, &createdAt)
	if err != nil {
		return nil, err
	}
	h.LastHeartbeat, _ = time.Parse(time.RFC3339, lastHb)
	h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &h, nil
}

// --- Threats ---

func (db *DB) CreateThreat(t *ThreatRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO threats (id, vm_id, user_id, type, level, description, evidence,
		screenshot, action, resolved, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.VMID, t.UserID, t.Type, t.Level, t.Description, t.Evidence,
		t.Screenshot, t.Action, boolToInt(t.Resolved),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetThreat(id string) (*ThreatRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, vm_id, user_id, type, level, description, evidence,
		screenshot, action, resolved, created_at, updated_at FROM threats WHERE id=?`, id)
	return scanThreat(row)
}

func (db *DB) ListThreats() ([]*ThreatRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, vm_id, user_id, type, level, description, evidence,
		screenshot, action, resolved, created_at, updated_at FROM threats ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threats []*ThreatRow
	for rows.Next() {
		t, err := scanThreat(rows)
		if err != nil {
			return nil, err
		}
		threats = append(threats, t)
	}
	return threats, rows.Err()
}

func (db *DB) ListUnresolvedThreats() ([]*ThreatRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, vm_id, user_id, type, level, description, evidence,
		screenshot, action, resolved, created_at, updated_at FROM threats WHERE resolved=0 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threats []*ThreatRow
	for rows.Next() {
		t, err := scanThreat(rows)
		if err != nil {
			return nil, err
		}
		threats = append(threats, t)
	}
	return threats, rows.Err()
}

func (db *DB) UpdateThreat(t *ThreatRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	t.UpdatedAt = time.Now()
	_, err := db.Exec(`UPDATE threats SET vm_id=?, user_id=?, type=?, level=?, description=?, evidence=?,
		screenshot=?, action=?, resolved=?, updated_at=? WHERE id=?`,
		t.VMID, t.UserID, t.Type, t.Level, t.Description, t.Evidence,
		t.Screenshot, t.Action, boolToInt(t.Resolved),
		t.UpdatedAt.Format(time.RFC3339), t.ID)
	return err
}

func scanThreat(s interface {
	Scan(dest ...interface{}) error
}) (*ThreatRow, error) {
	var (
		t                     ThreatRow
		createdAt, updatedAt  string
		resolved              int
	)
	err := s.Scan(&t.ID, &t.VMID, &t.UserID, &t.Type, &t.Level, &t.Description,
		&t.Evidence, &t.Screenshot, &t.Action, &resolved, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	t.Resolved = resolved != 0
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

// --- Queue ---

func (db *DB) CreateQueueEntry(q *QueueRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO queue (id, user_id, vm_id, status, priority, region, resource_class, position, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		q.ID, q.UserID, q.VMID, q.Status, q.Priority, q.Region, q.ResourceClass,
		q.Position, q.CreatedAt.Format(time.RFC3339), q.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) GetQueueEntry(id string) (*QueueRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, user_id, vm_id, status, priority, region, resource_class, position, created_at, updated_at
		FROM queue WHERE id=?`, id)
	return scanQueue(row)
}

func (db *DB) ListQueueByUser(userID string) ([]*QueueRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, vm_id, status, priority, region, resource_class, position, created_at, updated_at
		FROM queue WHERE user_id=? ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*QueueRow
	for rows.Next() {
		q, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, q)
	}
	return entries, rows.Err()
}

func (db *DB) ListQueueByStatus(status string) ([]*QueueRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, vm_id, status, priority, region, resource_class, position, created_at, updated_at
		FROM queue WHERE status=? ORDER BY priority DESC, created_at ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*QueueRow
	for rows.Next() {
		q, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, q)
	}
	return entries, rows.Err()
}

func (db *DB) UpdateQueueEntry(q *QueueRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	q.UpdatedAt = time.Now()
	_, err := db.Exec(`UPDATE queue SET user_id=?, vm_id=?, status=?, priority=?, region=?, resource_class=?, position=?, updated_at=?
		WHERE id=?`, q.UserID, q.VMID, q.Status, q.Priority, q.Region, q.ResourceClass, q.Position,
		q.UpdatedAt.Format(time.RFC3339), q.ID)
	return err
}

func (db *DB) DeleteQueueEntry(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`DELETE FROM queue WHERE id=?`, id)
	return err
}

func scanQueue(s interface {
	Scan(dest ...interface{}) error
}) (*QueueRow, error) {
	var (
		q                     QueueRow
		createdAt, updatedAt  string
	)
	err := s.Scan(&q.ID, &q.UserID, &q.VMID, &q.Status, &q.Priority, &q.Region,
		&q.ResourceClass, &q.Position, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	q.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	q.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &q, nil
}

// --- Audit Logs ---

func (db *DB) CreateAuditLog(a *AuditLogRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO audit_logs (id, session_id, user_id, action, details, ip_addr, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		a.ID, a.SessionID, a.UserID, a.Action, a.Details, a.IPAddr,
		a.CreatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) ListAuditLogsBySession(sessionID string) ([]*AuditLogRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, session_id, user_id, action, details, ip_addr, created_at
		FROM audit_logs WHERE session_id=? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*AuditLogRow
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (db *DB) ListAuditLogsByUser(userID string) ([]*AuditLogRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, session_id, user_id, action, details, ip_addr, created_at
		FROM audit_logs WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*AuditLogRow
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func scanAuditLog(s interface {
	Scan(dest ...interface{}) error
}) (*AuditLogRow, error) {
	var (
		l         AuditLogRow
		createdAt string
	)
	err := s.Scan(&l.ID, &l.SessionID, &l.UserID, &l.Action, &l.Details, &l.IPAddr, &createdAt)
	if err != nil {
		return nil, err
	}
	l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &l, nil
}

// --- Settings ---

func (db *DB) GetSetting(key string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (db *DB) SetSetting(key, val string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, val)
	return err
}

func (db *DB) GetAllSettings() (map[string]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// --- Storage ---

func (db *DB) GetStorageUsed(userID string) (int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var used int64
	err := db.QueryRow(`SELECT COALESCE(SUM(file_size), 0) FROM storage_files WHERE user_id=?`, userID).Scan(&used)
	return used, err
}

func (db *DB) AddStorageFile(id, userID, name, path, mimeType string, size int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO storage_files (id, user_id, name, path, mime_type, file_size, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`, id, userID, name, path, mimeType, size, now, now)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE users SET storage_used = (SELECT COALESCE(SUM(file_size), 0) FROM storage_files WHERE user_id=?) WHERE id=?`, userID, userID)
	return err
}

func (db *DB) DeleteStorageFile(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	var userID string
	err := db.QueryRow(`SELECT user_id FROM storage_files WHERE id=?`, id).Scan(&userID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM storage_files WHERE id=?`, id)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE users SET storage_used = (SELECT COALESCE(SUM(file_size), 0) FROM storage_files WHERE user_id=?) WHERE id=?`, userID, userID)
	return err
}

func (db *DB) ListStorageFiles(userID string) ([]map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, user_id, name, path, mime_type, file_size, created_at, updated_at
		FROM storage_files WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []map[string]interface{}
	for rows.Next() {
		var id, uid, name, path, mime, createdAt, updatedAt string
		var size int64
		if err := rows.Scan(&id, &uid, &name, &path, &mime, &size, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		files = append(files, map[string]interface{}{
			"id":        id,
			"name":      name,
			"mimeType":  mime,
			"fileSize":  size,
			"createdAt": createdAt,
		})
	}
	return files, rows.Err()
}

// --- helpers ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Preferences ---

func (db *DB) GetUserPreferences(userID string) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var prefs string
	err := db.QueryRow(`SELECT preferences FROM users WHERE id=?`, userID).Scan(&prefs)
	if err == sql.ErrNoRows {
		return []byte("{}"), nil
	}
	if err != nil {
		return nil, err
	}
	if prefs == "" {
		return []byte("{}"), nil
	}
	return []byte(prefs), nil
}

func (db *DB) SetUserPreferences(userID string, blob []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`UPDATE users SET preferences=?, updated_at=? WHERE id=?`,
		string(blob), time.Now().UTC().Format(time.RFC3339), userID)
	return err
}

// --- Reports ---

func (db *DB) CreateReport(r *ReportsRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if r.ID == "" {
		r.ID = fmt.Sprintf("rep_%d", time.Now().UnixNano())
	}
	ts := now()
	if r.CreatedAt == "" {
		r.CreatedAt = ts
	}
	r.UpdatedAt = ts
	if r.Status == "" {
		r.Status = "pending"
	}
	_, err := db.Exec(`INSERT INTO reports (id, reporter_id, target_type, target_id, reason, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ReporterID, r.TargetType, r.TargetID, r.Reason, r.Status, r.CreatedAt, r.UpdatedAt)
	return err
}

func (db *DB) ListReports() ([]*ReportsRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	rows, err := db.Query(`SELECT id, reporter_id, target_type, target_id, reason, status, reviewer_id, action, created_at, updated_at
		FROM reports ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*ReportsRow
	for rows.Next() {
		var r ReportsRow
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.TargetType, &r.TargetID, &r.Reason,
			&r.Status, &r.ReviewerID, &r.Action, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, &r)
	}
	return reports, rows.Err()
}

func (db *DB) GetReport(id string) (*ReportsRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	row := db.QueryRow(`SELECT id, reporter_id, target_type, target_id, reason, status, reviewer_id, action, created_at, updated_at
		FROM reports WHERE id=?`, id)
	var r ReportsRow
	err := row.Scan(&r.ID, &r.ReporterID, &r.TargetType, &r.TargetID, &r.Reason,
		&r.Status, &r.ReviewerID, &r.Action, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) UpdateReport(id, reviewerID, status, action string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`UPDATE reports SET reviewer_id=?, status=?, action=?, updated_at=? WHERE id=?`,
		reviewerID, status, action, now(), id)
	return err
}

// --- Personalization Sync Requests ---

func (db *DB) CreateSyncRequest(userID string) (string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	id := fmt.Sprintf("sync_%d", time.Now().UnixNano())
	ts := now()
	_, err := db.Exec(`INSERT INTO personalization_sync_requests (id, user_id, status, created_at, updated_at)
		VALUES (?,?,?,?,?)`, id, userID, "pending", ts, ts)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (db *DB) ApproveSyncRequest(id, reviewerID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(`UPDATE personalization_sync_requests SET status='approved', reviewer_id=?, updated_at=? WHERE id=?`,
		reviewerID, now(), id)
	return err
}

func init() {
	var _ = json.Marshal
}

func initDB() {
	log.Println("database package initialized")
}
