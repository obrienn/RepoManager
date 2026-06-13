package store

import (
	"context"
	"fmt"

	"repomanager/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateRepo(ctx context.Context, r *model.Repository) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO repositories (name, owner, github_url, local_path, description, license, default_branch)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		r.Name, r.Owner, r.GithubURL, r.LocalPath, r.Description, r.License, r.DefaultBranch,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create repo: %w", err)
	}
	return id, nil
}

func (s *Store) GetRepo(ctx context.Context, id int64) (*model.Repository, error) {
	r := &model.Repository{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, owner, github_url, local_path, description, license,
		        default_branch, last_updated, added_date, total_size_bytes,
		        needs_attention, attention_reason
		 FROM repositories WHERE id = $1`, id,
	).Scan(&r.ID, &r.Name, &r.Owner, &r.GithubURL, &r.LocalPath, &r.Description,
		&r.License, &r.DefaultBranch, &r.LastUpdated, &r.AddedDate,
		&r.TotalSizeBytes, &r.NeedsAttention, &r.AttentionReason)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get repo: %w", err)
	}
	return r, nil
}

func (s *Store) GetRepoByURL(ctx context.Context, url string) (*model.Repository, error) {
	r := &model.Repository{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, owner, github_url, local_path, description, license,
		        default_branch, last_updated, added_date, total_size_bytes,
		        needs_attention, attention_reason
		 FROM repositories WHERE github_url = $1`, url,
	).Scan(&r.ID, &r.Name, &r.Owner, &r.GithubURL, &r.LocalPath, &r.Description,
		&r.License, &r.DefaultBranch, &r.LastUpdated, &r.AddedDate,
		&r.TotalSizeBytes, &r.NeedsAttention, &r.AttentionReason)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get repo by url: %w", err)
	}
	return r, nil
}

func (s *Store) ListRepos(ctx context.Context, p model.ListParams) ([]model.RepoSummary, error) {
	query := `
		SELECT r.id, r.name, r.owner, r.github_url, r.description, r.license,
		       r.default_branch, r.last_updated, r.added_date, r.needs_attention
		FROM repositories r
		WHERE 1=1`
	args := []any{}

	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		query += fmt.Sprintf(` AND (r.name ILIKE $%d OR r.owner ILIKE $%d OR r.description ILIKE $%d
			OR r.readme_content ILIKE $%d
			OR EXISTS (SELECT 1 FROM topics t WHERE t.repo_id = r.id AND t.topic_name ILIKE $%d)
			OR EXISTS (SELECT 1 FROM repository_tags rt JOIN tags tg ON rt.tag_id = tg.id WHERE rt.repo_id = r.id AND tg.name ILIKE $%d))`,
			len(args), len(args), len(args), len(args), len(args), len(args))
	}

	switch p.Sort {
	case "name":
		query += " ORDER BY r.name"
	case "added_date":
		query += " ORDER BY r.added_date"
	case "last_updated":
		query += " ORDER BY r.last_updated"
	default:
		query += " ORDER BY r.added_date"
	}

	if p.Order == "asc" {
		query += " ASC"
	} else {
		query += " DESC"
	}

	if p.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", p.Limit)
	}
	if p.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", p.Offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	var repos []model.RepoSummary
	for rows.Next() {
		var r model.RepoSummary
		if err := rows.Scan(&r.ID, &r.Name, &r.Owner, &r.GithubURL, &r.Description,
			&r.License, &r.DefaultBranch, &r.LastUpdated, &r.AddedDate, &r.NeedsAttention); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		r.Topics = []string{}
		r.Tags = []model.Tag{}
		r.Languages = []model.Language{}
		repos = append(repos, r)
	}

	return repos, nil
}

func (s *Store) DeleteRepo(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
