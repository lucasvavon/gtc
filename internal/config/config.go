package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config structs
// ---------------------------------------------------------------------------

// ProviderBlock holds the VCS provider settings written to .gtc.yaml.
type ProviderBlock struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url,omitempty"`
	Token   string `yaml:"token"`
	Owner   string `yaml:"owner"`
	Repo    string `yaml:"repo"`
}

// WatchBlock holds the watch-mode settings written to .gtc.yaml.
type WatchBlock struct {
	Interval string   `yaml:"interval"`
	Events   []string `yaml:"events,omitempty"`
}

// ProjectConfig is the top-level structure of a .gtc.yaml file.
type ProjectConfig struct {
	Provider ProviderBlock `yaml:"provider"`
	Watch    WatchBlock    `yaml:"watch"`
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load finds the git root of the current working directory and reads the
// .gtc.yaml file located there. It returns a parsed ProjectConfig or an error.
func Load() (*ProjectConfig, error) {
	root, err := FindGitRoot()
	if err != nil {
		return nil, err
	}
	return LoadFrom(filepath.Join(root, ".gtc.yaml"))
}

// LoadFrom reads and parses the ProjectConfig from the file at path.
// It returns a descriptive error when the file is absent or malformed.
func LoadFrom(path string) (*ProjectConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s (run 'gtc init' to create it)", path)
		}
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()

	var cfg ProjectConfig
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// ---------------------------------------------------------------------------
// RunInteractiveInit
// ---------------------------------------------------------------------------

// RunInteractiveInit guides the user through creating a .gtc.yaml at the root
// of the current git repository. It pre-fills values from the git remote when
// possible and writes the result to <git-root>/.gtc.yaml.
func RunInteractiveInit(in io.Reader, out io.Writer) error {
	// 1. Must be inside a git repository.
	root, err := FindGitRoot()
	if err != nil {
		return fmt.Errorf("gtc init must be run inside a git repository: %w", err)
	}

	// 2. Try to pre-fill from the git remote (best-effort; ignore errors).
	var remote *RemoteInfo
	remote, _ = DetectRemote()

	r := bufio.NewReader(in)
	fmt.Fprintln(out, "gtc — interactive project setup")
	fmt.Fprintln(out, strings.Repeat("─", 42))

	// 3a. Provider name
	providerSuggestion := ""
	if remote != nil && remote.Provider != "" {
		providerSuggestion = remote.Provider
	}
	providerName := prompt(r, out, "Provider [github|gitlab|gitea|bitbucket]", providerSuggestion)

	// 3b. Base URL — only relevant for self-hosted or non-SaaS providers
	var baseURL string
	if needsBaseURL(providerName) {
		defaultBase := defaultBaseURL(providerName, remote)
		baseURL = prompt(r, out, "Base URL (e.g. https://gitlab.example.com)", defaultBase)
	}

	// 3c. Owner / organisation
	ownerSuggestion := ""
	if remote != nil {
		ownerSuggestion = remote.Owner
	}
	owner := prompt(r, out, "Owner / organisation", ownerSuggestion)

	// 3d. Repository name
	repoSuggestion := ""
	if remote != nil {
		repoSuggestion = remote.Repo
	}
	repo := prompt(r, out, "Repository name", repoSuggestion)

	// 3e. Token
	token := prompt(r, out, "Personal access token", "")

	// 3f. Watch interval
	interval := prompt(r, out, "Watch poll interval (e.g. 30s, 1m)", "1m")

	// 4. Build and write the config file.
	cfg := ProjectConfig{
		Provider: ProviderBlock{
			Name:    providerName,
			BaseURL: baseURL,
			Token:   token,
			Owner:   owner,
			Repo:    repo,
		},
		Watch: WatchBlock{
			Interval: interval,
		},
	}

	destPath := filepath.Join(root, ".gtc.yaml")
	if err := writeConfig(destPath, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n✓ Config written to %s\n", destPath)

	// 5. Remind the user to gitignore the file (token stored in plain text).
	fmt.Fprintln(out, "\n⚠  The file contains your token in plain text.")
	fmt.Fprintln(out, "   Add it to .gitignore to avoid committing credentials:")
	fmt.Fprintln(out, "     echo '.gtc.yaml' >> .gitignore")

	return nil
}

// ---------------------------------------------------------------------------
// RemoteInfo and git helpers
// ---------------------------------------------------------------------------

// RemoteInfo holds the parsed components of a git remote URL.
type RemoteInfo struct {
	Owner    string
	Repo     string
	Host     string
	Provider string // guessed from host, may be empty for unknown hosts
}

// FindGitRoot walks up the directory tree from the current working directory
// until it finds a directory containing a .git entry. It returns the absolute
// path of that directory or an error if none is found before reaching the
// filesystem root.
func FindGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not a git repository (or any parent up to filesystem root)")
		}
		dir = parent
	}
}

// DetectRemote runs `git remote get-url origin` in the git root directory,
// parses the resulting URL, and returns the structured RemoteInfo.
func DetectRemote() (*RemoteInfo, error) {
	root, err := FindGitRoot()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote get-url origin: %w", err)
	}

	rawURL := strings.TrimSpace(string(out))
	owner, repo, host, err := parseRemoteURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing remote URL %q: %w", rawURL, err)
	}

	return &RemoteInfo{
		Owner:    owner,
		Repo:     repo,
		Host:     host,
		Provider: guessProvider(host),
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func writeConfig(path string, cfg ProjectConfig) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return enc.Close()
}

// prompt prints a labelled prompt to out and reads a line from r.
// When the user submits an empty line, defaultVal is returned.
func prompt(r *bufio.Reader, out io.Writer, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(out, "  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(out, "  %s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// needsBaseURL returns true for providers where the base URL is not fixed
// and must be collected from the user.
func needsBaseURL(provider string) bool {
	switch provider {
	case "github", "bitbucket":
		return false
	default:
		// gitlab (may be self-hosted), gitea, or any unknown provider
		return true
	}
}

// defaultBaseURL returns a sensible default base URL for the provider,
// falling back to the detected remote host when available.
func defaultBaseURL(provider string, remote *RemoteInfo) string {
	if remote != nil && remote.Host != "" {
		return "https://" + remote.Host
	}
	switch provider {
	case "gitlab":
		return "https://gitlab.com"
	case "gitea":
		return "https://gitea.com"
	default:
		return ""
	}
}

// parseRemoteURL parses SSH (git@host:owner/repo.git) and HTTPS
// (https://host/owner/repo.git) git remote URLs.
func parseRemoteURL(raw string) (owner, repo, host string, err error) {
	if strings.HasPrefix(raw, "git@") {
		return parseSSHURL(raw)
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return parseHTTPSURL(raw)
	}
	return "", "", "", fmt.Errorf("unsupported URL scheme (expected git@ or https://): %q", raw)
}

func parseSSHURL(raw string) (owner, repo, host string, err error) {
	without := strings.TrimPrefix(raw, "git@")
	colonIdx := strings.Index(without, ":")
	if colonIdx < 0 {
		return "", "", "", fmt.Errorf("missing ':' in SSH URL %q", raw)
	}
	host = without[:colonIdx]
	owner, repo, err = splitOwnerRepo(without[colonIdx+1:])
	return owner, repo, host, err
}

func parseHTTPSURL(raw string) (owner, repo, host string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid HTTPS URL %q: %w", raw, err)
	}
	host = u.Hostname()
	owner, repo, err = splitOwnerRepo(strings.TrimPrefix(u.Path, "/"))
	return owner, repo, host, err
}

func splitOwnerRepo(path string) (owner, repo string, err error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected owner/repo in path %q", path)
	}
	return parts[0], parts[1], nil
}

// guessProvider maps well-known git hosting hostnames to provider identifiers.
func guessProvider(host string) string {
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	switch host {
	case "github.com":
		return "github"
	case "gitlab.com":
		return "gitlab"
	case "bitbucket.org":
		return "bitbucket"
	default:
		return ""
	}
}
