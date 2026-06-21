package s3

import "aws-terminal/internal/ui/pageapi"

import "testing"

func TestPageStatusPrefersErrorsOverActivity(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.loadingBuckets = true
	page.bucketErr = "bucket failure"

	status := page.PageStatus(pageapi.State{})
	if status.Error != "bucket failure" {
		t.Fatalf("expected bucket error, got %#v", status)
	}
	if status.Message != "" {
		t.Fatalf("expected no message when error is present, got %#v", status)
	}
}

func TestPageStatusReportsSyncActivity(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.syncing = true

	status := page.PageStatus(pageapi.State{})
	if status.Message == "" {
		t.Fatalf("expected sync status message, got %#v", status)
	}
}
