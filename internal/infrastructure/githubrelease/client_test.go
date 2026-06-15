package githubrelease

import "testing"

func TestNewClientConfiguresGitHubClient(t *testing.T) {
	client := NewClient("owner", "repo")
	if client == nil || client.client == nil {
		t.Fatal("NewClient should configure a non-nil go-github client")
	}
}
