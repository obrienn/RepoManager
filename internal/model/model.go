package model

import "time"

type Repository struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Owner           string     `json:"owner"`
	GithubURL       string     `json:"github_url"`
	LocalPath       string     `json:"local_path"`
	Description     *string    `json:"description"`
	License         *string    `json:"license"`
	DefaultBranch   string     `json:"default_branch"`
	LastUpdated     *time.Time `json:"last_updated"`
	AddedDate       time.Time  `json:"added_date"`
	TotalSizeBytes  int64      `json:"total_size_bytes"`
	NeedsAttention  bool       `json:"needs_attention"`
	AttentionReason *string    `json:"attention_reason"`
}

type RepoSummary struct {
	ID             int64       `json:"id"`
	Name           string      `json:"name"`
	Owner          string      `json:"owner"`
	GithubURL      string      `json:"github_url"`
	Description    *string     `json:"description"`
	License        *string     `json:"license"`
	DefaultBranch  string      `json:"default_branch"`
	LastUpdated    *time.Time  `json:"last_updated"`
	AddedDate      time.Time   `json:"added_date"`
	NeedsAttention bool        `json:"needs_attention"`
	Topics         []string    `json:"topics"`
	Tags           []Tag       `json:"tags"`
	Languages      []Language  `json:"languages"`
	LatestRelease  *Release    `json:"latest_release"`
}

type CreateRepoRequest struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	GithubURL string `json:"github_url"`
}

type Language struct {
	Name       string  `json:"language_name"`
	Percentage float64 `json:"percentage"`
	Bytes      int64   `json:"bytes"`
}

type Topic struct {
	Name string `json:"topic_name"`
}

type Release struct {
	TagName      string     `json:"tag_name"`
	PublishedAt  *time.Time `json:"published_date"`
	IsPrerelease bool       `json:"is_prerelease"`
}

type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ListParams struct {
	Sort   string // name, added_date, last_updated
	Order  string // asc, desc
	Search string
	Topic  string
	Tag    string
	Offset int
	Limit  int
}
