package gitops

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// EnsureSSHPushURL gives HTTPS origins an SSH push URL without changing credentials.
// Read the configured URL directly because remote get-url expands insteadOf rules.
func EnsureSSHPushURL(repo string) error {
	config, err := runGit(repo, "config", "--null", "--list")
	if err != nil {
		return fmt.Errorf("reading remote configuration: %w", err)
	}
	var origin string
	foundOrigin := false
	for entry := range strings.SplitSeq(config, "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		switch key {
		case "remote.origin.pushurl":
			return nil
		case "remote.origin.url":
			// Git fetches from the first URL when an origin has several.
			if !foundOrigin {
				origin = value
				foundOrigin = true
			}
		}
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" {
		return nil
	}
	if parsed.Hostname() == "" || strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("HTTPS origin must include a host and repository path")
	}
	path := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".git") + ".git"
	_, err = runGit(repo, "config", "--local", "remote.origin.pushurl", "git@"+parsed.Hostname()+":"+path)
	return err
}

// CanonicalRemoteIdentity normalizes a Git remote for identity comparison.
// Network remotes use lowercase host plus a case-preserving repository path.
func CanonicalRemoteIdentity(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n\t ") {
		return "", fmt.Errorf("invalid Git remote %q", value)
	}

	if !strings.Contains(value, "://") {
		if at := strings.IndexByte(value, '@'); at > 0 {
			if colon := strings.IndexByte(value[at+1:], ':'); colon >= 0 {
				colon += at + 1
				return canonicalNetworkRemote(value[at+1:colon], value[colon+1:])
			}
		}
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parsing Git remote: %w", err)
	}
	switch parsed.Scheme {
	case "http", "https", "ssh", "git":
		host := parsed.Hostname()
		if port := parsed.Port(); port != "" {
			host += ":" + port
		}
		return canonicalNetworkRemote(host, parsed.Path)
	case "file":
		if parsed.Path == "" {
			return "", fmt.Errorf("file Git remote has no path")
		}
		path, err := filepath.Abs(filepath.Clean(parsed.Path))
		if err != nil {
			return "", fmt.Errorf("resolving file Git remote: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		return "file:" + path, nil
	default:
		return "", fmt.Errorf("unsupported Git remote scheme %q", parsed.Scheme)
	}
}

func canonicalNetworkRemote(host, repositoryPath string) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	repositoryPath = strings.Trim(strings.TrimSpace(repositoryPath), "/")
	repositoryPath = strings.TrimSuffix(repositoryPath, ".git")
	if host == "" || repositoryPath == "" {
		return "", fmt.Errorf("git remote must include host and repository path")
	}
	return host + "/" + repositoryPath, nil
}
