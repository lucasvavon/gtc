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
	defer func() { _ = f.Close() }()

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

// RunInteractiveInit creates a .gtc.yaml at the root of the current git
// repository. It attempts to auto-detect provider, owner, repo, and token
// from the git remote and environment variables.
//
// Fast path — when the remote and a token are both detected automatically,
// the user only needs to confirm (or pass autoYes=true to skip even that).
//
// Interactive path — whenever information is missing the user is prompted;
// detected values are offered as defaults so a single Enter suffices.
func RunInteractiveInit(in io.Reader, out io.Writer, autoYes bool) error {
	// 1. Must be inside a git repository.
	root, err := FindGitRoot()
	if err != nil {
		return fmt.Errorf("gtc init must be run inside a git repository: %w", err)
	}

	destPath := filepath.Join(root, ".gtc.yaml")

	// 2. Detect remote and token from the environment.
	remote, _ := DetectRemote()
	detectedToken, tokenEnv := "", ""
	if remote != nil {
		detectedToken, tokenEnv = detectToken(remote.Provider)
	}

	// ── Fast path ────────────────────────────────────────────────────────────
	// Everything we need is available without user input.
	if remote != nil && remote.Provider != "" && remote.Owner != "" &&
		remote.Repo != "" && detectedToken != "" {

		printAutoSummary(out, remote, tokenEnv, destPath)

		confirmed := autoYes
		if !confirmed {
			r := bufio.NewReader(in)
			ans := prompt(r, out, "Créer .gtc.yaml avec ces paramètres ? [Y/n]", "Y")
			confirmed = strings.ToLower(strings.TrimSpace(ans)) != "n"
		}

		if confirmed {
			cfg := buildProjectConfig(
				remote.Provider,
				defaultBaseURL(remote.Provider, remote),
				remote.Owner, remote.Repo,
				detectedToken, "1m",
			)
			return writeAndReport(out, destPath, cfg)
		}

		// User declined → fall through to interactive with pre-filled values.
		_, _ = fmt.Fprintln(out, "")
	}

	// ── Interactive path ──────────────────────────────────────────────────────
	r := bufio.NewReader(in)
	_, _ = fmt.Fprintln(out, "gtc — interactive project setup")
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 42))

	providerDefault := ""
	if remote != nil {
		providerDefault = remote.Provider
	}
	providerName := prompt(r, out, "Provider [github|gitlab|gitea|bitbucket]", providerDefault)

	var baseURL string
	if needsBaseURL(providerName) {
		baseURL = prompt(r, out, "Base URL (e.g. https://gitlab.example.com)", defaultBaseURL(providerName, remote))
	}

	ownerDefault := ""
	if remote != nil {
		ownerDefault = remote.Owner
	}
	owner := prompt(r, out, "Owner / organisation", ownerDefault)

	repoDefault := ""
	if remote != nil {
		repoDefault = remote.Repo
	}
	repo := prompt(r, out, "Repository name", repoDefault)

	// Token: show which env var to use if one was found, use it as silent default.
	tokenLabel := "Personal access token"
	if tokenEnv != "" {
		tokenLabel = fmt.Sprintf("Personal access token [Enter to use $%s]", tokenEnv)
	}
	tokenInput := promptRaw(r, out, tokenLabel)
	token := tokenInput
	if token == "" {
		token = detectedToken
	}

	interval := prompt(r, out, "Watch poll interval (e.g. 30s, 1m)", "1m")

	cfg := buildProjectConfig(providerName, baseURL, owner, repo, token, interval)
	return writeAndReport(out, destPath, cfg)
}

// ── Auto-detect helpers ───────────────────────────────────────────────────────

// detectToken looks for a provider token in well-known environment variables.
// Returns the token value and the env var it came from.
func detectToken(provider string) (token, envVar string) {
	for _, name := range tokenEnvVars(provider) {
		if v := os.Getenv(name); v != "" {
			return v, name
		}
	}
	return "", ""
}

// tokenEnvVars returns the candidate environment variable names for a provider.
func tokenEnvVars(provider string) []string {
	switch provider {
	case "github":
		return []string{"GITHUB_TOKEN", "GH_TOKEN"}
	case "gitlab":
		return []string{"GITLAB_TOKEN", "GL_TOKEN"}
	case "gitea":
		return []string{"GITEA_TOKEN"}
	case "bitbucket":
		return []string{"BITBUCKET_TOKEN", "BITBUCKET_APP_PASSWORD"}
	default:
		return nil
	}
}

// printAutoSummary prints the auto-detected configuration before confirmation.
func printAutoSummary(out io.Writer, remote *RemoteInfo, tokenEnv, destPath string) {
	_, _ = fmt.Fprintln(out, "gtc — configuration automatique")
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 42))
	_, _ = fmt.Fprintf(out, "  Provider  :  %s\n", remote.Provider)
	if remote.Host != "" && needsBaseURL(remote.Provider) {
		_, _ = fmt.Fprintf(out, "  Base URL  :  https://%s\n", remote.Host)
	}
	_, _ = fmt.Fprintf(out, "  Owner     :  %s\n", remote.Owner)
	_, _ = fmt.Fprintf(out, "  Repo      :  %s\n", remote.Repo)
	_, _ = fmt.Fprintf(out, "  Token     :  $%s ✓\n", tokenEnv)
	_, _ = fmt.Fprintf(out, "  Interval  :  1m (défaut)\n")
	_, _ = fmt.Fprintf(out, "  Dest      :  %s\n", destPath)
	_, _ = fmt.Fprintln(out, "")
}

// buildProjectConfig assembles a ProjectConfig from individual fields.
func buildProjectConfig(provider, baseURL, owner, repo, token, interval string) ProjectConfig {
	return ProjectConfig{
		Provider: ProviderBlock{
			Name:    provider,
			BaseURL: baseURL,
			Token:   token,
			Owner:   owner,
			Repo:    repo,
		},
		Watch: WatchBlock{Interval: interval},
	}
}

// writeAndReport writes the config file and prints confirmation + gitignore tip.
func writeAndReport(out io.Writer, destPath string, cfg ProjectConfig) error {
	if err := writeConfig(destPath, cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "\n✓ Config written to %s\n", destPath)
	_, _ = fmt.Fprintln(out, "\n⚠  The file contains your token in plain text.")
	_, _ = fmt.Fprintln(out, "   Add it to .gitignore to avoid committing credentials:")
	_, _ = fmt.Fprintln(out, "     echo '.gtc.yaml' >> .gitignore")
	return nil
}

// promptRaw prints a label and reads a line without showing a default value.
// Used for sensitive fields like tokens.
func promptRaw(r *bufio.Reader, out io.Writer, label string) string {
	_, _ = fmt.Fprintf(out, "  %s: ", label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
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
	defer func() { _ = f.Close() }()

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
		_, _ = fmt.Fprintf(out, "  %s [%s]: ", label, defaultVal)
	} else {
		_, _ = fmt.Fprintf(out, "  %s: ", label)
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
