package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"testing"

	"aws-terminal/internal/config"
)

type memoryPreferenceStore struct {
	prefs config.Preferences
	saves int
}

func (s *memoryPreferenceStore) Load() (config.Preferences, error) {
	return s.prefs, nil
}

func (s *memoryPreferenceStore) Save(prefs config.Preferences) error {
	s.prefs = prefs
	s.saves++
	return nil
}

func TestNewModelWithPreferencesCreatesPreferenceFile(t *testing.T) {
	store := &memoryPreferenceStore{}
	_ = NewModelWithPreferences(nil, nil, testPageRegistry(), store)
	if store.saves != 1 {
		t.Fatalf("saves = %d, want startup save", store.saves)
	}
}

func TestNewModelWithPreferencesSelectsLastPageAndRegion(t *testing.T) {
	store := &memoryPreferenceStore{prefs: config.Preferences{LastPage: "cloudfront", LastRegion: "ap-southeast-2"}}
	model := NewModelWithPreferences(nil, nil, testPageRegistry(), store)

	if got := model.currentPageID(); got != "cloudfront" {
		t.Fatalf("current page = %q, want cloudfront", got)
	}
	if got := model.selectedRegion; got != "ap-southeast-2" {
		t.Fatalf("selected region = %q, want ap-southeast-2", got)
	}
}

func testPageRegistry() []pageapi.Page {
	return []pageapi.Page{
		&testPage{id: "dashboard"},
		&testPage{id: "s3-buckets"},
		&testPage{id: "cloudfront"},
	}
}

func TestRememberPreferencesSavesNonEmptyValues(t *testing.T) {
	store := &memoryPreferenceStore{}
	model := Model{preferenceStore: store}

	model.rememberProfile(" prod ")
	model.rememberRegion(" eu-west-1 ")
	model.rememberPage(" s3-buckets ")

	if store.prefs.LastProfile != "prod" {
		t.Fatalf("LastProfile = %q", store.prefs.LastProfile)
	}
	if store.prefs.LastRegion != "eu-west-1" {
		t.Fatalf("LastRegion = %q", store.prefs.LastRegion)
	}
	if store.prefs.LastPage != "s3-buckets" {
		t.Fatalf("LastPage = %q", store.prefs.LastPage)
	}
	if store.saves != 3 {
		t.Fatalf("saves = %d, want 3", store.saves)
	}
}
