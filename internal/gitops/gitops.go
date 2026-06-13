package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RepoInfo struct {
	Description   string
	DefaultBranch string
	Languages     []LanguageInfo
	License       string
	LatestRelease *ReleaseInfo
	FileCount     int
	TotalSize     int64
	ReadmeContent string
}

type LanguageInfo struct {
	Name       string
	Percentage float64
	Bytes      int64
}

type ReleaseInfo struct {
	TagName      string
	PublishedAt  time.Time
	IsPrerelease bool
}

func Clone(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "50", url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone: %s: %w", string(out), err)
	}
	exec.CommandContext(ctx, "git", "-C", dest, "lfs", "pull").Run()
	return nil
}

func Pull(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull: %s: %w", string(out), err)
	}
	exec.CommandContext(ctx, "git", "-C", path, "lfs", "pull").Run()
	return nil
}

func GetReadme(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("readdir: %w", err)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), "readme.md") || strings.EqualFold(e.Name(), "README") {
			data, err := os.ReadFile(filepath.Join(path, e.Name()))
			if err != nil {
				return "", fmt.Errorf("readfile: %w", err)
			}
			return string(data), nil
		}
	}
	return "", nil
}

func FileCount(path string) (int, error) {
	cmd := exec.Command("git", "-C", path, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ls-files: %w", err)
	}
	if len(out) == 0 {
		return 0, nil
	}
	return strings.Count(string(out), "\x00"), nil
}

func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func DetectLanguages(path string) ([]LanguageInfo, error) {
	cmd := exec.Command("git", "-C", path, "ls-files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}

	type langStat struct {
		name  string
		bytes int64
	}
	langMap := make(map[string]int64)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, file := range lines {
		if file == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file))
		lang := extToLang(ext)
		if lang == "" {
			continue
		}
		info, err := os.Lstat(filepath.Join(path, file))
		if err != nil {
			continue
		}
		langMap[lang] += info.Size()
	}

	var langs []langStat
	for name, bytes := range langMap {
		langs = append(langs, langStat{name, bytes})
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].bytes > langs[j].bytes })

	var totalBytes int64
	for _, l := range langs {
		totalBytes += l.bytes
	}

	var result []LanguageInfo
	for _, l := range langs {
		pct := 0.0
		if totalBytes > 0 {
			pct = float64(l.bytes) / float64(totalBytes) * 100
		}
		result = append(result, LanguageInfo{
			Name:       l.name,
			Bytes:      l.bytes,
			Percentage: pct,
		})
	}
	return result, nil
}

func DetectLicense(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasPrefix(name, "license") || strings.HasPrefix(name, "licence") || name == "copying" {
			data, err := os.ReadFile(filepath.Join(path, e.Name()))
			if err != nil {
				continue
			}
			return detectLicenseFromContent(string(data))
		}
	}
	return ""
}

func DetectReleases(ctx context.Context, path string) (*ReleaseInfo, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "tag", "--sort=-creatordate")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "") {
		return nil, nil
	}

	latest := tags[0]
	isPre := strings.Contains(strings.ToLower(latest), "rc") ||
		strings.Contains(strings.ToLower(latest), "alpha") ||
		strings.Contains(strings.ToLower(latest), "beta") ||
		strings.Contains(strings.ToLower(latest), "pre")

	dateCmd := exec.CommandContext(ctx, "git", "-C", path, "log", "-1", "--format=%cI", latest)
	dateOut, err := dateCmd.Output()
	if err != nil {
		return nil, nil
	}
	publishedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(string(dateOut)))

	return &ReleaseInfo{
		TagName:      latest,
		PublishedAt:  publishedAt,
		IsPrerelease: isPre,
	}, nil
}

func DefaultBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "main", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func Analyze(ctx context.Context, path string) (*RepoInfo, error) {
	info := &RepoInfo{}

	branch, err := DefaultBranch(path)
	if err == nil {
		info.DefaultBranch = branch
	}

	readme, err := GetReadme(path)
	if err == nil {
		info.ReadmeContent = readme
	}

	langs, err := DetectLanguages(path)
	if err == nil {
		info.Languages = langs
	}

	info.License = DetectLicense(path)

	release, err := DetectReleases(ctx, path)
	if err == nil {
		info.LatestRelease = release
	}

	count, err := FileCount(path)
	if err == nil {
		info.FileCount = count
	}

	size, err := DirSize(path)
	if err == nil {
		info.TotalSize = size
	}

	return info, nil
}

func extToLang(ext string) string {
	langs := map[string]string{
		".go":          "Go",
		".rs":          "Rust",
		".py":          "Python",
		".js":          "JavaScript",
		".ts":          "TypeScript",
		".tsx":         "TypeScript",
		".jsx":         "JavaScript",
		".c":           "C",
		".cpp":         "C++",
		".cc":          "C++",
		".cxx":         "C++",
		".h":           "C",
		".hpp":         "C++",
		".java":        "Java",
		".rb":          "Ruby",
		".php":         "PHP",
		".cs":          "C#",
		".swift":       "Swift",
		".kt":          "Kotlin",
		".scala":       "Scala",
		".dart":        "Dart",
		".lua":         "Lua",
		".r":           "R",
		".m":           "Objective-C",
		".mm":          "Objective-C++",
		".pl":          "Perl",
		".pm":          "Perl",
		".hs":          "Haskell",
		".elm":         "Elm",
		".ex":          "Elixir",
		".exs":         "Elixir",
		".erl":         "Erlang",
		".clj":         "Clojure",
		".cljs":        "ClojureScript",
		".jl":          "Julia",
		".nim":         "Nim",
		".zig":         "Zig",
		".sh":          "Shell",
		".bash":        "Shell",
		".zsh":         "Shell",
		".ps1":         "PowerShell",
		".html":        "HTML",
		".htm":         "HTML",
		".css":         "CSS",
		".scss":        "SCSS",
		".sass":        "Sass",
		".less":        "Less",
		".sql":         "SQL",
		".xml":         "XML",
		".json":        "JSON",
		".yaml":        "YAML",
		".yml":         "YAML",
		".toml":        "TOML",
		".ini":         "INI",
		".cfg":         "Config",
		".md":          "Markdown",
		".markdown":    "Markdown",
		".rst":         "reStructuredText",
		".tex":         "TeX",
		".dockerfile":  "Dockerfile",
		".cmake":       "CMake",
		".makefile":    "Makefile",
		".mk":          "Makefile",
		".vue":         "Vue",
		".svelte":      "Svelte",
		".tf":          "HCL",
		".proto":       "Protocol Buffers",
		".graphql":     "GraphQL",
		".gql":         "GraphQL",
		".wasm":        "WebAssembly",
		".csv":         "CSV",
	}
	if lang, ok := langs[ext]; ok {
		return lang
	}

	full := map[string]string{
		".ipynb":        "Jupyter Notebook",
	}
	if lang, ok := full[ext]; ok {
		return lang
	}
	return ""
}

func detectLicenseFromContent(content string) string {
	content = strings.ToLower(content)
	patterns := map[string]string{
		"gnu general public license v3":     "GPL-3.0",
		"gnu general public license v2":     "GPL-2.0",
		"gnu general public license":        "GPL",
		"gnu lesser general public license": "LGPL",
		"gnu affero general public license": "AGPL",
		"apache license, version 2.0":         "Apache-2.0",
		"apache license":                    "Apache",
		"mit license":                       "MIT",
		"the mit license":                   "MIT",
		"bsd 3-clause":                      "BSD-3-Clause",
		"bsd 2-clause":                      "BSD-2-Clause",
		"bsd license":                       "BSD",
		"mozilla public license":            "MPL",
		"the mozilla public license":        "MPL",
		"creative commons":                  "CC",
		"unlicense":                         "Unlicense",
		"do what the fuck":                  "WTFPL",
		"isc license":                       "ISC",
	}
	for pattern, license := range patterns {
		if strings.Contains(content, pattern) {
			return license
		}
	}
	return "Other"
}
