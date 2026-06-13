package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RepoMeta struct {
	Description string   `json:"description"`
	Topics      []string `json:"topics"`
	License     string   `json:"license"`
}

type apiRepo struct {
	Description *string  `json:"description"`
	Topics      []string `json:"topics"`
	License     *apiLicense `json:"license"`
}

type apiLicense struct {
	SPDXID string `json:"spdx_id"`
}

func FetchRepoMeta(ctx context.Context, owner, name, token string) (*RepoMeta, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RepoManager")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &RepoMeta{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var api apiRepo
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	meta := &RepoMeta{
		Topics: api.Topics,
	}
	if meta.Topics == nil {
		meta.Topics = []string{}
	}
	if api.Description != nil {
		meta.Description = *api.Description
	}
	if api.License != nil {
		meta.License = strings.TrimSpace(api.License.SPDXID)
	}

	return meta, nil
}
