package gitops

// UnpushedCommits lists commits absent from every local remote-tracking branch.
// It deliberately does not fetch, so deletion works without network access.
func UnpushedCommits(repo, branch string) (string, error) {
	return runGit(repo, "rev-list", "refs/heads/"+branch, "--not", "--remotes")
}

// DiscardWorktreeChanges preserves HEAD while discarding tracked and untracked files.
func DiscardWorktreeChanges(path string) error {
	if _, err := runGit(path, "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	_, err := runGit(path, "clean", "-fd")
	return err
}

// RebaseUnsigned preserves unsigned records even when the clone enables signing.
func RebaseUnsigned(path, base string) error {
	_, err := runGit(path, "-c", "commit.gpgsign=false", "rebase", "--no-gpg-sign", base)
	return err
}

// FastForward updates the base checkout without creating a merge commit.
func FastForward(path, branch string) error {
	_, err := runGit(path, "merge", "--ff-only", "--no-edit", "refs/heads/"+branch)
	return err
}

// RestorePaths rolls back files staged by a failed tool-owned commit.
func RestorePaths(path string, paths ...string) error {
	_, err := runGit(path, append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)...)
	return err
}
