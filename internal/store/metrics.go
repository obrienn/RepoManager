package store

import (
	"context"
	"fmt"
	"time"
)

type Metrics struct {
	TotalRepos     int64     `json:"total_repos"`
	NeedAttention  int64     `json:"need_attention"`
	TotalSizeGB    float64   `json:"total_size_gb"`
	NextUpdateTime time.Time `json:"next_update_time"`
}

func (s *Store) GetMetrics(ctx context.Context, nextUpdate time.Time) (*Metrics, error) {
	m := &Metrics{NextUpdateTime: nextUpdate}
	var totalBytes int64
	err := s.pool.QueryRow(ctx,
		`SELECT
			COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE needs_attention)::bigint,
			COALESCE(SUM(total_size_bytes), 0)::bigint
		 FROM repositories`,
	).Scan(&m.TotalRepos, &m.NeedAttention, &totalBytes)
	if err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	m.TotalSizeGB = float64(totalBytes) / 1024 / 1024 / 1024
	return m, nil
}
