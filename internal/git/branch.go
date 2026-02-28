package git

func (r *Runner) BranchDelete(name string) error {
	_, err := r.run("branch", "-D", name)
	return err
}

func (r *Runner) CurrentBranch() (string, error) {
	return r.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (r *Runner) Merge(branch string) (string, error) {
	return r.run("merge", branch, "--no-ff", "-m", "Merge "+branch)
}

func (r *Runner) Checkout(branch string) error {
	_, err := r.run("checkout", branch)
	return err
}
