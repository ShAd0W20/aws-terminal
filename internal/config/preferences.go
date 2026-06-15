package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const appDirName = "aws-terminal"

type Preferences struct {
	LastProfile            string   `json:"lastProfile,omitempty"`
	LastRegion             string   `json:"lastRegion,omitempty"`
	LastPage               string   `json:"lastPage,omitempty"`
	S3SourceDirectory      string   `json:"s3SourceDirectory,omitempty"`
	S3RecentPrefixes       []string `json:"s3RecentPrefixes,omitempty"`
	CheckForUpdatesOnStart *bool    `json:"checkForUpdatesOnStart,omitempty"`
}

type PreferenceStore interface {
	Load() (Preferences, error)
	Save(Preferences) error
}

type FilePreferenceStore struct {
	path string
	mu   sync.Mutex
}

func NewFilePreferenceStore() (*FilePreferenceStore, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	return NewFilePreferenceStoreAt(filepath.Join(home, ".config", appDirName, "config.json")), nil
}

func NewFilePreferenceStoreAt(path string) *FilePreferenceStore {
	return &FilePreferenceStore{path: path}
}

func (s *FilePreferenceStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FilePreferenceStore) Load() (Preferences, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return Preferences{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Preferences{}, nil
		}
		return Preferences{}, fmt.Errorf("read preferences: %w", err)
	}

	var prefs Preferences
	if err := json.Unmarshal(payload, &prefs); err != nil {
		return Preferences{}, fmt.Errorf("parse preferences: %w", err)
	}
	return normalizePreferences(prefs), nil
}

func (s *FilePreferenceStore) Save(prefs Preferences) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	prefs = normalizePreferences(prefs)
	payload, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create preferences directory: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return fmt.Errorf("write preferences: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace preferences: %w", err)
	}
	return nil
}

func normalizePreferences(prefs Preferences) Preferences {
	prefs.LastProfile = strings.TrimSpace(prefs.LastProfile)
	prefs.LastRegion = strings.TrimSpace(prefs.LastRegion)
	prefs.LastPage = strings.TrimSpace(prefs.LastPage)
	prefs.S3SourceDirectory = strings.TrimSpace(prefs.S3SourceDirectory)
	prefs.S3RecentPrefixes = compactStrings(prefs.S3RecentPrefixes, 10)
	return prefs
}

func compactStrings(values []string, limit int) []string {
	seen := map[string]struct{}{}
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		compacted = append(compacted, value)
		if limit > 0 && len(compacted) >= limit {
			break
		}
	}
	return compacted
}

func CheckForUpdatesOnStart(prefs Preferences) bool {
	return prefs.CheckForUpdatesOnStart == nil || *prefs.CheckForUpdatesOnStart
}

func WithCheckForUpdatesOnStart(prefs Preferences, enabled bool) Preferences {
	prefs.CheckForUpdatesOnStart = &enabled
	return normalizePreferences(prefs)
}

func RememberRecentPrefix(prefs Preferences, prefix string) Preferences {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return normalizePreferences(prefs)
	}
	prefs.S3RecentPrefixes = append([]string{prefix}, prefs.S3RecentPrefixes...)
	return normalizePreferences(prefs)
}
