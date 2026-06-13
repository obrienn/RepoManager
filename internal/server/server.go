package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"repomanager/internal/config"
	"repomanager/internal/gitops"
	"repomanager/internal/handler"
	"repomanager/internal/store"
)

func Run(cfg *config.Config) error {
	ctx := context.Background()

	if err := ensureGitEnvironment(); err != nil {
		return err
	}

	db, err := store.New(ctx, cfg.DatabaseURL())
	if err != nil {
		return err
	}
	defer db.Close()

	sql, err := os.ReadFile("migrations/001_schema.sql")
	if err != nil {
		return err
	}
	if err := db.Migrate(ctx, string(sql)); err != nil {
		return err
	}

	var nextUpdate time.Time
	interval, _ := time.ParseDuration(cfg.UpdateInterval)

	go handler.ScanDatastore(db, cfg)

	go backfillReadmes(db)

	startBackgroundUpdater(db, cfg, &nextUpdate, interval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handler.Health(db))
	mux.HandleFunc("POST /api/repos", handler.AddRepo(db, cfg))
	mux.HandleFunc("GET /api/repos", handler.ListRepos(db))
	mux.HandleFunc("GET /api/repos/{id}", handler.GetRepo(db))
	mux.HandleFunc("DELETE /api/repos/{id}", handler.DeleteRepo(db))
	mux.HandleFunc("GET /api/repos/{id}/readme", handler.Readme(db))
	mux.HandleFunc("POST /api/repos/{id}/update", handler.UpdateRepoByID(db, cfg))
	mux.HandleFunc("POST /api/repos/{id}/reclone", handler.RecloneRepo(db, cfg))
	mux.HandleFunc("POST /api/repos/update-all", handler.UpdateAll(db, cfg, &nextUpdate))
	mux.HandleFunc("GET /api/tags", handler.ListTags(db))
	mux.HandleFunc("POST /api/tags", handler.CreateTag(db))
	mux.HandleFunc("PUT /api/tags/{id}", handler.UpdateTag(db))
	mux.HandleFunc("DELETE /api/tags/{id}", handler.DeleteTag(db))
	mux.HandleFunc("GET /api/repos/{id}/tags", handler.GetRepoTags(db))
	mux.HandleFunc("PUT /api/repos/{id}/tags", handler.SetRepoTags(db))
	mux.HandleFunc("GET /api/metrics", handler.Metrics(db, &nextUpdate))

	fileServer := http.FileServer(http.Dir("frontend"))
	mux.Handle("/", fileServer)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr(),
		Handler: mux,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	return httpServer.ListenAndServe()
}

func startBackgroundUpdater(s *store.Store, cfg *config.Config, nextUpdate *time.Time, interval time.Duration) {
	if cfg.UpdateInterval == "0" {
		return
	}

	if interval > 0 {
		*nextUpdate = time.Now().Add(interval)
		log.Printf("background updater: interval %s, next update at %s", cfg.UpdateInterval, nextUpdate.Format(time.RFC3339))
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				log.Printf("background updater: starting cycle")
				handler.RunUpdateCycle(s, cfg, nextUpdate)
			}
		}()
		return
	}

	go scheduleDaily(s, cfg, nextUpdate, 3, 0)
}

func scheduleDaily(s *store.Store, cfg *config.Config, nextUpdate *time.Time, hour, min int) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		*nextUpdate = next
		log.Printf("background updater: next update at %s", next.Format(time.RFC3339))

		<-time.After(next.Sub(now))

		log.Printf("background updater: starting cycle")
		handler.RunUpdateCycle(s, cfg, nextUpdate)
	}
}

func ensureGitEnvironment() error {
	exec.Command("git", "config", "--global", "--add", "safe.directory", "*").Run()
	exec.Command("git", "lfs", "install").Run()
	return nil
}

func backfillReadmes(s *store.Store) {
	ids, err := s.GetAllRepos(context.Background())
	if err != nil {
		log.Printf("backfill: get repos: %v", err)
		return
	}
	for _, id := range ids {
		repo, err := s.GetRepo(context.Background(), id)
		if err != nil || repo == nil {
			continue
		}
		hasContent, _ := s.RepoHasReadmeContent(context.Background(), id)
		if hasContent {
			continue
		}
		content, err := gitops.GetReadme(repo.LocalPath)
		if err != nil || content == "" {
			continue
		}
		if err := s.SaveReadme(context.Background(), id, content); err != nil {
			log.Printf("backfill %s/%s: %v", repo.Owner, repo.Name, err)
		} else {
			log.Printf("backfill %s/%s: ok", repo.Owner, repo.Name)
		}
	}
}
