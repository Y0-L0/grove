package gitops

// TreePaths lists committed paths under a directory without reading its checkout.
func TreePaths(repo, ref, path string) (string, error) {
	return runGit(repo, "ls-tree", "-r", "--name-only", ref, "--", path)
}

// FileAt reads a committed file without changing the checkout.
func FileAt(repo, ref, path string) (string, error) {
	return runGit(repo, "show", ref+":"+path)
}
