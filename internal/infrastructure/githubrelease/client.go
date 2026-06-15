package githubrelease

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v88/github"

	domainupdate "aws-terminal/internal/domain/update"
)

type Client struct {
	owner  string
	repo   string
	client *github.Client
}

func NewClient(owner, repo string) *Client {
	client, err := github.NewClient(github.WithTimeout(10 * time.Second))
	if err != nil {
		return &Client{owner: owner, repo: repo}
	}
	return &Client{owner: owner, repo: repo, client: client}
}

func (c *Client) LatestRelease(ctx context.Context) (domainupdate.Release, error) {
	if c == nil || c.client == nil || c.owner == "" || c.repo == "" {
		return domainupdate.Release{}, fmt.Errorf("github release client is not configured")
	}

	release, _, err := c.client.Repositories.GetLatestRelease(ctx, c.owner, c.repo)
	if err != nil {
		return domainupdate.Release{}, fmt.Errorf("get latest GitHub release: %w", err)
	}
	if release == nil || release.GetTagName() == "" {
		return domainupdate.Release{}, fmt.Errorf("latest GitHub release has no tag")
	}

	assets := make([]domainupdate.Asset, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if asset == nil || asset.GetName() == "" || asset.GetBrowserDownloadURL() == "" {
			continue
		}
		assets = append(assets, domainupdate.Asset{Name: asset.GetName(), URL: asset.GetBrowserDownloadURL()})
	}

	return domainupdate.Release{
		Version: release.GetTagName(),
		URL:     release.GetHTMLURL(),
		Assets:  assets,
	}, nil
}
