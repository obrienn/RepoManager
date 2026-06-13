package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"repomanager/internal/model"
)

func (s *Store) UpdateRepoMeta(ctx context.Context, r *model.Repository) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE repositories
		 SET description = $2, license = $3, default_branch = $4,
		     last_updated = $5, total_size_bytes = $6, needs_attention = $7, attention_reason = $8
		 WHERE id = $1`,
		r.ID, r.Description, r.License, r.DefaultBranch,
		r.LastUpdated, r.TotalSizeBytes, r.NeedsAttention, r.AttentionReason,
	)
	return err
}

func (s *Store) SetLanguages(ctx context.Context, repoID int64, langs []model.Language) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM languages WHERE repo_id = $1`, repoID)
	if err != nil {
		return fmt.Errorf("clear languages: %w", err)
	}
	for _, l := range langs {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO languages (repo_id, language_name, percentage, bytes)
			 VALUES ($1, $2, $3, $4)`,
			repoID, l.Name, l.Percentage, l.Bytes)
		if err != nil {
			return fmt.Errorf("insert language: %w", err)
		}
	}
	return nil
}

func (s *Store) SetTopics(ctx context.Context, repoID int64, topics []string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM topics WHERE repo_id = $1`, repoID)
	if err != nil {
		return fmt.Errorf("clear topics: %w", err)
	}
	for _, t := range topics {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO topics (repo_id, topic_name) VALUES ($1, $2)`,
			repoID, t)
		if err != nil {
			return fmt.Errorf("insert topic: %w", err)
		}
	}
	return nil
}

func (s *Store) SetRelease(ctx context.Context, repoID int64, r *model.Release) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM releases WHERE repo_id = $1`, repoID)
	if err != nil {
		return fmt.Errorf("clear releases: %w", err)
	}
	if r != nil {
		_, err = s.pool.Exec(ctx,
			`INSERT INTO releases (repo_id, tag_name, published_date, is_prerelease)
			 VALUES ($1, $2, $3, $4)`,
			repoID, r.TagName, r.PublishedAt, r.IsPrerelease)
		if err != nil {
			return fmt.Errorf("insert release: %w", err)
		}
	}
	return nil
}

func (s *Store) GetRepoLanguages(ctx context.Context, repoID int64) ([]model.Language, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT language_name, percentage, bytes FROM languages WHERE repo_id = $1 ORDER BY percentage DESC`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var langs []model.Language
	for rows.Next() {
		var l model.Language
		if err := rows.Scan(&l.Name, &l.Percentage, &l.Bytes); err != nil {
			return nil, err
		}
		langs = append(langs, l)
	}
	return langs, nil
}

func (s *Store) GetRepoTopics(ctx context.Context, repoID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT topic_name FROM topics WHERE repo_id = $1`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func (s *Store) RepoHasReadmeContent(ctx context.Context, repoID int64) (bool, error) {
	var content *string
	err := s.pool.QueryRow(ctx, `SELECT readme_content FROM repositories WHERE id = $1`, repoID).Scan(&content)
	if err != nil {
		return false, err
	}
	return content != nil && *content != "", nil
}

func (s *Store) SaveReadme(ctx context.Context, repoID int64, content string) error {
	_, err := s.pool.Exec(ctx, `UPDATE repositories SET readme_content = $2 WHERE id = $1`, repoID, content)
	return err
}

func (s *Store) GetRepoLatestRelease(ctx context.Context, repoID int64) (*model.Release, error) {
	r := &model.Release{}
	err := s.pool.QueryRow(ctx,
		`SELECT tag_name, published_date, is_prerelease FROM releases WHERE repo_id = $1`, repoID,
	).Scan(&r.TagName, &r.PublishedAt, &r.IsPrerelease)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r, nil
}
