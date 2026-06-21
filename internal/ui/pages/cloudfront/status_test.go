package cloudfront

import "aws-terminal/internal/ui/pageapi"

import "testing"

func TestPageStatusPrefersErrorsOverActivity(t *testing.T) {
	page := NewCloudFrontPage(staticCloudFrontService{})
	page.loading = true
	page.loadErr = "load failure"

	status := page.PageStatus(pageapi.State{})
	if status.Error != "load failure" {
		t.Fatalf("expected load error, got %#v", status)
	}
	if status.Message != "" {
		t.Fatalf("expected no message when error is present, got %#v", status)
	}
}

func TestPageStatusReportsInvalidationActivity(t *testing.T) {
	page := NewCloudFrontPage(staticCloudFrontService{})
	page.creating = true

	status := page.PageStatus(pageapi.State{})
	if status.Message == "" {
		t.Fatalf("expected invalidation status message, got %#v", status)
	}
}
