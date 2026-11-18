package gitops

// CommitSignatures reports signing status for commits unique to a workspace branch.
func CommitSignatures(path, base, branch string) (string, error) {
	return runGit(path, "log", "--format=%G?", base+"..refs/heads/"+branch)
}

// SignCommits autosquashes fixups and signs rewritten commits, including merges.
func SignCommits(path, base string) error {
	_, err := runGit(path, "-c", "sequence.editor=true", "rebase", "--interactive", "--rebase-merges", "--autosquash", "--force-rebase", "--gpg-sign", "--exec", "git commit --amend --no-edit -S", base)
	return err
}

// PushWorkspaceBranch pushes only the explicitly named branch and sets its upstream.
func PushWorkspaceBranch(path, branch string) error {
	ref := "refs/heads/" + branch
	_, err := runGit(path, "push", "--force-with-lease", "--set-upstream", "origin", ref+":"+ref)
	return err
}
