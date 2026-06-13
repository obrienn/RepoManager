package store

import (
	"context"
	"fmt"
	"time"

	"repomanager/internal/gitops"
)

type UpdateLogRow struct {
	ID           int64
	RepoID       int64
	CheckedAt    time.Time
	Status       string
	FileCountOld *int
	FileCountNew *int
	SizeBytesOld *int64
	SizeBytesNew *int64
	DeltaPercent *float64
	Flagged      bool
}

func (s *Store) InsertUpdateLog(ctx context.Context, repoID int64, result *gitops.UpdateResult) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO update_logs (repo_id, status, file_count_old, file_count_new, size_bytes_old, size_bytes_new, delta_percent, flagged)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		repoID, result.Status,
		result.PrevFileCount, result.NewFileCount,
		result.PrevSize, result.NewSize,
		result.DeltaPercent,
		result.Status == "flagged",
	)
	return fmt.Errorf("insert update log: %w", err)
}

func (s *Store) GetAllRepos(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM repositories ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("get all repos: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
