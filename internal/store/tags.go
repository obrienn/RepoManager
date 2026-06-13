package store

import (
	"context"
	"fmt"

	"repomanager/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateTag(ctx context.Context, name, color string) (*model.Tag, error) {
	t := &model.Tag{Name: name, Color: color}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tags (name, color) VALUES ($1, $2) RETURNING id`, name, color,
	).Scan(&t.ID)
	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return t, nil
}

func (s *Store) GetTag(ctx context.Context, id int64) (*model.Tag, error) {
	t := &model.Tag{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, color FROM tags WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Color)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return t, nil
}

func (s *Store) ListTags(ctx context.Context) ([]model.Tag, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, color FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Store) UpdateTag(ctx context.Context, id int64, name, color string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE tags SET name = $2, color = $3 WHERE id = $1`, id, name, color)
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteTag(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetRepoTags(ctx context.Context, repoID int64, tagIDs []int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM repository_tags WHERE repo_id = $1`, repoID)
	if err != nil {
		return fmt.Errorf("clear repo tags: %w", err)
	}
	for _, tid := range tagIDs {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO repository_tags (repo_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			repoID, tid)
		if err != nil {
			return fmt.Errorf("insert repo tag: %w", err)
		}
	}
	return nil
}

func (s *Store) GetRepoTags(ctx context.Context, repoID int64) ([]model.Tag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.name, t.color FROM tags t
		 JOIN repository_tags rt ON t.id = rt.tag_id
		 WHERE rt.repo_id = $1 ORDER BY t.name`, repoID)
	if err != nil {
		return nil, fmt.Errorf("get repo tags: %w", err)
	}
	defer rows.Close()
	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}
