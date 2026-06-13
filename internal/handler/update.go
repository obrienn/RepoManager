package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"repomanager/internal/config"
	"repomanager/internal/github"
	"repomanager/internal/gitops"
	"repomanager/internal/model"
	"repomanager/internal/store"
)

func UpdateAll(s *store.Store, cfg *config.Config, nextUpdate *time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "update started"})

		go func() {
			RunUpdateCycle(s, cfg, nextUpdate)
		}()
	}
}

func RunUpdateCycle(s *store.Store, cfg *config.Config, nextUpdate *time.Time) {
	ScanDatastore(s, cfg)
	ids, err := s.GetAllRepos(context.Background())
	if err != nil {
		log.Printf("update-all: get repos: %v", err)
		return
	}
	for _, id := range ids {
		updateRepoByID(s, cfg, id)
	}
	log.Printf("update-all: complete (%d repos)", len(ids))

	if nextUpdate != nil && cfg.UpdateInterval != "0" && cfg.UpdateInterval != "" {
		d, err := time.ParseDuration(cfg.UpdateInterval)
		if err == nil {
			t := time.Now().Add(d)
			*nextUpdate = t
		}
	}
}

func ScanDatastore(s *store.Store, cfg *config.Config) {
	discovered, err := gitops.ScanDirectory(cfg.LibraryPath)
	if err != nil {
		log.Printf("scan datastore: %v", err)
		return
	}

	imported := 0
	for _, d := range discovered {
		existing, err := s.GetRepoByURL(context.Background(), d.GithubURL)
		if err != nil {
			log.Printf("scan check %s/%s: %v", d.Owner, d.Name, err)
			continue
		}
		if existing != nil {
			if existing.LocalPath != d.LocalPath {
				existing.LocalPath = d.LocalPath
				s.UpdateRepoMeta(context.Background(), existing)
			}
			continue
		}

		repo := &model.Repository{
			Name:          d.Name,
			Owner:         d.Owner,
			GithubURL:     d.GithubURL,
			LocalPath:     d.LocalPath,
			DefaultBranch: "main",
		}

		id, err := s.CreateRepo(context.Background(), repo)
		if err != nil {
			log.Printf("scan import %s/%s: %v", d.Owner, d.Name, err)
			continue
		}
		repo.ID = id

		info, err := gitops.Analyze(context.Background(), d.LocalPath)
		if err != nil {
			log.Printf("scan analyze %s/%s: %v", d.Owner, d.Name, err)
			repo.NeedsAttention = true
			reason := "analyze failed: " + err.Error()
			repo.AttentionReason = &reason
			s.UpdateRepoMeta(context.Background(), repo)
			continue
		}

		if info.License != "" {
			repo.License = &info.License
		}
		repo.DefaultBranch = info.DefaultBranch
		now := time.Now()
		repo.LastUpdated = &now
		repo.TotalSizeBytes = info.TotalSize
		s.UpdateRepoMeta(context.Background(), repo)

		if info.ReadmeContent != "" {
			s.SaveReadme(context.Background(), repo.ID, info.ReadmeContent)
		}

		var langs []model.Language
		for _, l := range info.Languages {
			langs = append(langs, model.Language{Name: l.Name, Percentage: l.Percentage, Bytes: l.Bytes})
		}
		s.SetLanguages(context.Background(), repo.ID, langs)

		if info.LatestRelease != nil {
			s.SetRelease(context.Background(), repo.ID, &model.Release{
				TagName:      info.LatestRelease.TagName,
				PublishedAt:  &info.LatestRelease.PublishedAt,
				IsPrerelease: info.LatestRelease.IsPrerelease,
			})
		}

		meta, err := github.FetchRepoMeta(context.Background(), d.Owner, d.Name, cfg.GithubToken)
		if err != nil {
			log.Printf("scan github meta %s/%s: %v", d.Owner, d.Name, err)
		} else {
			if meta.Description != "" {
				repo.Description = &meta.Description
				s.UpdateRepoMeta(context.Background(), repo)
			}
			if meta.License != "" && repo.License == nil {
				repo.License = &meta.License
				s.UpdateRepoMeta(context.Background(), repo)
			}
			if len(meta.Topics) > 0 {
				s.SetTopics(context.Background(), repo.ID, meta.Topics)
			}
		}

		imported++
		log.Printf("scan imported %s/%s", d.Owner, d.Name)
	}

	if imported > 0 {
		log.Printf("scan complete: %d new repos imported out of %d discovered", imported, len(discovered))
	}
}

func UpdateRepoByID(s *store.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "update started"})

		go func() {
			updateRepoByID(s, cfg, id)
		}()
	}
}

func updateRepoByID(s *store.Store, cfg *config.Config, id int64) {
	repo, err := s.GetRepo(context.Background(), id)
	if err != nil || repo == nil {
		log.Printf("update repo %d: get: %v", id, err)
		return
	}

	if repo.NeedsAttention {
		log.Printf("update repo %s/%s: skipping (needs attention)", repo.Owner, repo.Name)
		return
	}

	if _, err := os.Stat(repo.LocalPath); os.IsNotExist(err) {
		repo.NeedsAttention = true
		reason := "local clone missing"
		repo.AttentionReason = &reason
		s.UpdateRepoMeta(context.Background(), repo)
		log.Printf("update repo %s/%s: clone missing, flagged", repo.Owner, repo.Name)
		return
	}

	if !gitops.CheckRemoteExists(context.Background(), repo.GithubURL) {
		repo.NeedsAttention = true
		reason := "remote unreachable (repo may be private or deleted)"
		repo.AttentionReason = &reason
		s.UpdateRepoMeta(context.Background(), repo)
		log.Printf("update repo %s/%s: remote unreachable, flagged", repo.Owner, repo.Name)
		s.InsertUpdateLog(context.Background(), id, &gitops.UpdateResult{
			Status: "flagged",
			Error:  "remote unreachable",
		})
		return
	}

	result := gitops.Update(context.Background(), repo.LocalPath, 30.0)
	s.InsertUpdateLog(context.Background(), id, result)

	if result.Status == "error" {
		repo.NeedsAttention = true
		reason := "update error: " + result.Error
		repo.AttentionReason = &reason
		s.UpdateRepoMeta(context.Background(), repo)
		return
	}

	if result.Status == "flagged" {
		repo.NeedsAttention = true
		reason := "significant file reduction after pull, reverted"
		repo.AttentionReason = &reason
		s.UpdateRepoMeta(context.Background(), repo)
		return
	}

	info, err := gitops.Analyze(context.Background(), repo.LocalPath)
	if err != nil {
		log.Printf("update repo %s/%s: analyze: %v", repo.Owner, repo.Name, err)
		return
	}

	if info.License != "" {
		repo.License = &info.License
	}
	repo.DefaultBranch = info.DefaultBranch
	now := time.Now()
	repo.LastUpdated = &now
	repo.TotalSizeBytes = info.TotalSize
	repo.NeedsAttention = false
	repo.AttentionReason = nil

	s.UpdateRepoMeta(context.Background(), repo)

	if info.ReadmeContent != "" {
		s.SaveReadme(context.Background(), repo.ID, info.ReadmeContent)
	}

	var langs []model.Language
	for _, l := range info.Languages {
		langs = append(langs, model.Language{Name: l.Name, Percentage: l.Percentage, Bytes: l.Bytes})
	}
	s.SetLanguages(context.Background(), repo.ID, langs)

	if info.LatestRelease != nil {
		s.SetRelease(context.Background(), repo.ID, &model.Release{
			TagName:      info.LatestRelease.TagName,
			PublishedAt:  &info.LatestRelease.PublishedAt,
			IsPrerelease: info.LatestRelease.IsPrerelease,
		})
	}

	meta, err := github.FetchRepoMeta(context.Background(), repo.Owner, repo.Name, cfg.GithubToken)
	if err != nil {
		log.Printf("update repo %s/%s: github meta: %v", repo.Owner, repo.Name, err)
	} else {
		if meta.Description != "" {
			repo.Description = &meta.Description
			s.UpdateRepoMeta(context.Background(), repo)
		}
		if meta.License != "" && repo.License == nil {
			repo.License = &meta.License
			s.UpdateRepoMeta(context.Background(), repo)
		}
		if len(meta.Topics) > 0 {
			s.SetTopics(context.Background(), repo.ID, meta.Topics)
		}
	}

	log.Printf("update repo %s/%s: done", repo.Owner, repo.Name)
}
