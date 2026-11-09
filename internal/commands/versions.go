package commands

// VerifiedVersions lists the server versions verified by this build. The test
// checks this small manifest against the snapshot filenames without shipping them.
func VerifiedVersions() []string {
	return []string{"2.31.10"}
}
