//go:build !windows

package output

func downloadInUse(string, error) bool { return false }
