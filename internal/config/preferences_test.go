package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewFilePreferenceStoreUsesDotConfigPath(t *testing.T) {
	store, err := NewFilePreferenceStore()
	if err != nil {
		t.Fatalf("NewFilePreferenceStore() error = %v", err)
	}
	if filepath.Base(store.Path()) != "config.json" {
		t.Fatalf("path = %q, want config.json basename", store.Path())
	}
	if filepath.Base(filepath.Dir(store.Path())) != appDirName {
		t.Fatalf("path = %q, want %s directory", store.Path(), appDirName)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(store.Path()))) != ".config" {
		t.Fatalf("path = %q, want .config parent", store.Path())
	}
}

func TestFilePreferenceStoreLoadMissingFileReturnsEmpty(t *testing.T) {
	store := NewFilePreferenceStoreAt(filepath.Join(t.TempDir(), "config.json"))
	prefs, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(prefs, Preferences{}) {
		t.Fatalf("prefs = %#v, want empty", prefs)
	}
}

func TestFilePreferenceStoreSaveAndLoad(t *testing.T) {
	store := NewFilePreferenceStoreAt(filepath.Join(t.TempDir(), "aws-terminal", "config.json"))
	want := Preferences{
		LastProfile:       " prod ",
		LastRegion:        " eu-west-1 ",
		LastPage:          " s3-buckets ",
		S3SourceDirectory: " /tmp/site ",
		S3RecentPrefixes:  []string{" site ", "", "site", "assets"},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expected := Preferences{
		LastProfile:       "prod",
		LastRegion:        "eu-west-1",
		LastPage:          "s3-buckets",
		S3SourceDirectory: "/tmp/site",
		S3RecentPrefixes:  []string{"site", "assets"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("prefs = %#v, want %#v", got, expected)
	}
}

func TestRememberRecentPrefixMovesPrefixToFront(t *testing.T) {
	prefs := RememberRecentPrefix(Preferences{S3RecentPrefixes: []string{"assets", "old"}}, "/site/")
	prefs = RememberRecentPrefix(prefs, "assets")

	want := []string{"assets", "site", "old"}
	if !reflect.DeepEqual(prefs.S3RecentPrefixes, want) {
		t.Fatalf("recent prefixes = %#v, want %#v", prefs.S3RecentPrefixes, want)
	}
}

func TestCheckForUpdatesOnStartDefaultsEnabled(t *testing.T) {
	if !CheckForUpdatesOnStart(Preferences{}) {
		t.Fatal("missing preference should default to enabled")
	}

	prefs := WithCheckForUpdatesOnStart(Preferences{}, false)
	if CheckForUpdatesOnStart(prefs) {
		t.Fatal("explicit false should disable startup update checks")
	}
}
