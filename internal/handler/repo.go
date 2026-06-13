package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yuin/goldmark"

	"repomanager/internal/config"
	"repomanager/internal/github"
	"repomanager/internal/gitops"
	"repomanager/internal/model"
	"repomanager/internal/store"
)

func AddRepo(s *store.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.CreateRepoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.Owner = strings.TrimSpace(req.Owner)
		req.GithubURL = strings.TrimSpace(req.GithubURL)

		if req.Name == "" || req.Owner == "" || req.GithubURL == "" {
			writeError(w, http.StatusBadRequest, "name, owner, and github_url are required")
			return
		}
		if !isValidGithubURL(req.GithubURL) {
			writeError(w, http.StatusBadRequest, "invalid github URL. expected: https://github.com/owner/name")
			return
		}

		existing, err := s.GetRepoByURL(r.Context(), req.GithubURL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if existing != nil {
			writeError(w, http.StatusConflict, "repository already exists in catalog")
			return
		}

		localPath := filepath.Join(cfg.LibraryPath, req.Owner, req.Name)
		repo := &model.Repository{
			Name:          req.Name,
			Owner:         req.Owner,
			GithubURL:     req.GithubURL,
			DefaultBranch: "main",
			LocalPath:     localPath,
		}

		id, err := s.CreateRepo(r.Context(), repo)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		repo.ID = id

		go enrichRepo(s, cfg, repo)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(repo)
	}
}

func enrichRepo(s *store.Store, cfg *config.Config, repo *model.Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("cloning %s/%s to %s", repo.Owner, repo.Name, repo.LocalPath)
	if err := gitops.Clone(ctx, repo.GithubURL, repo.LocalPath); err != nil {
		log.Printf("clone %s/%s: %v", repo.Owner, repo.Name, err)
		repo.NeedsAttention = true
		reason := fmt.Sprintf("clone failed: %v", err)
		repo.AttentionReason = &reason
		s.UpdateRepoMeta(ctx, repo)
		return
	}

	info, err := gitops.Analyze(ctx, repo.LocalPath)
	if err != nil {
		log.Printf("analyze %s/%s: %v", repo.Owner, repo.Name, err)
		return
	}

	if info.License != "" {
		repo.License = &info.License
	}
	repo.DefaultBranch = info.DefaultBranch
	now := time.Now()
	repo.LastUpdated = &now
	repo.TotalSizeBytes = info.TotalSize

	s.UpdateRepoMeta(ctx, repo)

	if info.ReadmeContent != "" {
		s.SaveReadme(ctx, repo.ID, info.ReadmeContent)
	}

	var langs []model.Language
	for _, l := range info.Languages {
		langs = append(langs, model.Language{Name: l.Name, Percentage: l.Percentage, Bytes: l.Bytes})
	}
	s.SetLanguages(ctx, repo.ID, langs)

	if info.LatestRelease != nil {
		s.SetRelease(ctx, repo.ID, &model.Release{
			TagName:      info.LatestRelease.TagName,
			PublishedAt:  &info.LatestRelease.PublishedAt,
			IsPrerelease: info.LatestRelease.IsPrerelease,
		})
	}

	meta, err := github.FetchRepoMeta(context.Background(), repo.Owner, repo.Name, cfg.GithubToken)
	if err != nil {
		log.Printf("github meta %s/%s: %v", repo.Owner, repo.Name, err)
	} else {
		if meta.Description != "" {
			repo.Description = &meta.Description
			s.UpdateRepoMeta(ctx, repo)
		}
		if meta.License != "" && repo.License == nil {
			repo.License = &meta.License
			s.UpdateRepoMeta(ctx, repo)
		}
		if len(meta.Topics) > 0 {
			s.SetTopics(ctx, repo.ID, meta.Topics)
		}
	}

	log.Printf("enriched %s/%s", repo.Owner, repo.Name)
}

func ListRepos(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}

		params := model.ListParams{
			Sort:   q.Get("sort"),
			Order:  q.Get("order"),
			Search: q.Get("search"),
			Topic:  q.Get("topic"),
			Tag:    q.Get("tag"),
			Limit:  limit,
			Offset: offset,
		}

		repos, err := s.ListRepos(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if repos == nil {
			repos = []model.RepoSummary{}
		}

		for i := range repos {
			langs, _ := s.GetRepoLanguages(r.Context(), repos[i].ID)
			if langs != nil {
				repos[i].Languages = langs
			}
			topics, _ := s.GetRepoTopics(r.Context(), repos[i].ID)
			if topics != nil {
				repos[i].Topics = topics
			}
			rel, _ := s.GetRepoLatestRelease(r.Context(), repos[i].ID)
			if rel != nil {
				repos[i].LatestRelease = rel
			}
			tags, _ := s.GetRepoTags(r.Context(), repos[i].ID)
			if tags != nil {
				repos[i].Tags = tags
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	}
}

func GetRepo(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		repo, err := s.GetRepo(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if repo == nil {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repo)
	}
}

func DeleteRepo(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		repo, err := s.GetRepo(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if repo == nil {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}

		if err := os.RemoveAll(repo.LocalPath); err != nil {
			log.Printf("delete repo %s/%s: remove local copy: %v", repo.Owner, repo.Name, err)
		}

		if err := s.DeleteRepo(r.Context(), id); err != nil {
			if err == pgx.ErrNoRows {
				writeError(w, http.StatusNotFound, "repository not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}

func RecloneRepo(s *store.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "reclone started"})

		go func() {
			repo, err := s.GetRepo(context.Background(), id)
			if err != nil || repo == nil {
				log.Printf("reclone %d: get: %v", id, err)
				return
			}

			if !gitops.CheckRemoteExists(context.Background(), repo.GithubURL) {
				repo.NeedsAttention = true
				reason := "reclone failed: remote unreachable (repo may be private or deleted)"
				repo.AttentionReason = &reason
				s.UpdateRepoMeta(context.Background(), repo)
				log.Printf("reclone %s/%s: remote unreachable, flagged", repo.Owner, repo.Name)
				return
			}

			prevCount, _, _, err := gitops.Snapshot(context.Background(), repo.LocalPath)
			if err != nil {
				prevCount = 0
			}

			if err := os.RemoveAll(repo.LocalPath); err != nil {
				log.Printf("reclone %s/%s: remove old clone: %v", repo.Owner, repo.Name, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if err := gitops.Clone(ctx, repo.GithubURL, repo.LocalPath); err != nil {
				repo.NeedsAttention = true
				reason := fmt.Sprintf("reclone failed: %v", err)
				repo.AttentionReason = &reason
				s.UpdateRepoMeta(context.Background(), repo)
				log.Printf("reclone %s/%s: clone failed: %v", repo.Owner, repo.Name, err)
				return
			}

			info, err := gitops.Analyze(context.Background(), repo.LocalPath)
			if err != nil {
				repo.NeedsAttention = true
				reason := fmt.Sprintf("reclone analyze failed: %v", err)
				repo.AttentionReason = &reason
				s.UpdateRepoMeta(context.Background(), repo)
				log.Printf("reclone %s/%s: analyze: %v", repo.Owner, repo.Name, err)
				return
			}

			if prevCount > 0 {
				delta := float64(info.FileCount-prevCount) / float64(prevCount) * 100
				if delta < -30 {
					repo.NeedsAttention = true
					reason := fmt.Sprintf("reclone: significant file reduction (%.1f%%)", delta)
					repo.AttentionReason = &reason
					s.UpdateRepoMeta(context.Background(), repo)
					log.Printf("reclone %s/%s: flagged (delta %.1f%%)", repo.Owner, repo.Name, delta)
					return
				}
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
				log.Printf("reclone github meta %s/%s: %v", repo.Owner, repo.Name, err)
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

			log.Printf("reclone %s/%s: done", repo.Owner, repo.Name)
		}()
	}
}

func Readme(s *store.Store) http.HandlerFunc {
	md := goldmark.New()
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		repo, err := s.GetRepo(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if repo == nil {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}

		content, err := gitops.GetReadme(repo.LocalPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var buf bytes.Buffer
		if err := md.Convert([]byte(content), &buf); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	}
}

func isValidGithubURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" || u.Host != "github.com" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
