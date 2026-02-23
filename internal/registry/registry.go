package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Project holds configuration for a single project.
type Project struct {
	WorkDir string `json:"workDir"`
}

// TelegramConfig holds Telegram-specific configuration.
type TelegramConfig struct {
	Token        string            `json:"token"`
	AdminChatIDs []int64           `json:"adminChatIDs"`
	ChatBindings map[string]string `json:"chatBindings"` // chatID → project name
}

// LinearConfig holds Linear-specific configuration.
type LinearConfig struct {
	APIKey        string            `json:"apiKey"`
	WebhookSecret string            `json:"webhookSecret"`
	WebhookPort   int               `json:"webhookPort"`
	TriggerState  string            `json:"triggerState"`
	DoneState     string            `json:"doneState"`
	TeamBindings  map[string]string `json:"teamBindings"` // teamKey → project name
}

// GoogleConfig holds Gmail OAuth2 credentials and tokens.
type GoogleConfig struct {
	Email        string `json:"email"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RefreshToken string `json:"refreshToken"`
	AccessToken  string `json:"accessToken,omitempty"`
	TokenExpiry  string `json:"tokenExpiry,omitempty"` // RFC3339
}

// TwilioConfig holds Twilio REST API credentials.
type TwilioConfig struct {
	AccountSID  string `json:"accountSID"`
	AuthToken   string `json:"authToken"`
	PhoneNumber string `json:"phoneNumber"`
}

// BrowserConfig holds browser automation settings.
type BrowserConfig struct {
	Headless bool `json:"headless"`
}

// Data is the full contents of projects.json.
type Data struct {
	Projects       map[string]Project `json:"projects"`
	DefaultProject string             `json:"defaultProject,omitempty"`
	Telegram       *TelegramConfig    `json:"telegram,omitempty"`
	Linear         *LinearConfig      `json:"linear,omitempty"`
	Google         *GoogleConfig      `json:"google,omitempty"`
	Twilio         *TwilioConfig      `json:"twilio,omitempty"`
	Browser        *BrowserConfig     `json:"browser,omitempty"`
}

// Registry holds the parsed projects.json and supports hot-reload.
type Registry struct {
	mu    sync.RWMutex
	path  string
	data  Data
	mtime time.Time
}

// Load reads and parses the projects.json file at path.
func Load(path string) (*Registry, error) {
	r := &Registry{path: path}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) reload() error {
	f, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("open %s: %w", r.path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", r.path, err)
	}

	var data Data
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return fmt.Errorf("parse %s: %w", r.path, err)
	}

	if data.Projects == nil {
		data.Projects = make(map[string]Project)
	}

	// Ensure workDirs exist for all projects.
	for name, p := range data.Projects {
		if p.WorkDir != "" {
			if err := os.MkdirAll(p.WorkDir, 0755); err != nil {
				log.Printf("registry: create workDir for %q (%s): %v", name, p.WorkDir, err)
			}
		}
	}

	r.mu.Lock()
	r.data = data
	r.mtime = info.ModTime()
	r.mu.Unlock()
	return nil
}

// Watch polls the projects.json file for changes every 5 seconds and reloads on mtime change.
func (r *Registry) Watch(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(r.path)
			if err != nil {
				continue
			}
			r.mu.RLock()
			mtime := r.mtime
			r.mu.RUnlock()
			if info.ModTime().After(mtime) {
				if err := r.reload(); err != nil {
					log.Printf("registry: reload error: %v", err)
				} else {
					log.Printf("registry: reloaded %s", r.path)
				}
			}
		}
	}
}

// Get returns a snapshot of the current registry data.
func (r *Registry) Get() Data {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data
}

// WorkDir returns the working directory for the named project, or "" if not found.
func (r *Registry) WorkDir(project string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.data.Projects[project]; ok {
		return p.WorkDir
	}
	return ""
}

// DefaultProject returns the name of the default project, or "" if unset.
func (r *Registry) DefaultProject() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data.DefaultProject
}

// ProjectForChat returns the project name bound to chatID, if any.
func (r *Registry) ProjectForChat(chatID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data.Telegram == nil || r.data.Telegram.ChatBindings == nil {
		return "", false
	}
	p, ok := r.data.Telegram.ChatBindings[chatID]
	return p, ok
}

// ProjectForTeam returns the project name bound to a Linear team key, if any.
func (r *Registry) ProjectForTeam(teamKey string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data.Linear == nil || r.data.Linear.TeamBindings == nil {
		return "", false
	}
	p, ok := r.data.Linear.TeamBindings[teamKey]
	return p, ok
}

// IsAdminChat returns true if chatID is in the admin list.
func (r *Registry) IsAdminChat(chatID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data.Telegram == nil {
		return false
	}
	for _, id := range r.data.Telegram.AdminChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

// Save applies mutate to the current data and writes it atomically to disk.
func (r *Registry) Save(mutate func(*Data)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mutate(&r.data)

	b, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	// Update mtime so Watch doesn't trigger a redundant reload.
	if info, err := os.Stat(r.path); err == nil {
		r.mtime = info.ModTime()
	}

	return nil
}
