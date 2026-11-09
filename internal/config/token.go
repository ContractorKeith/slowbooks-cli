package config

import (
	"io"
	"os"
	"runtime"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func CheckMode(mode os.FileMode, goos string) bool { return goos == "windows" || mode.Perm() == 0600 }

// TokenFile validates the opened file and bounds the credential read.
func TokenFile(path string) (string, error) {
	// Stat rejects known FIFOs; the nonblocking open closes the replacement race.
	info, e := os.Stat(path)
	if e != nil {
		return "", fileError("read token file", path, e)
	}
	if !info.Mode().IsRegular() {
		return "", &output.Error{Code: "auth", Message: "token file is not a regular file: " + path}
	}
	f, e := openToken(path)
	if e != nil {
		return "", fileError("read token file", path, e)
	}
	defer f.Close()
	info, e = f.Stat()
	if e != nil {
		return "", fileError("inspect token file", path, e)
	}
	if !info.Mode().IsRegular() || !CheckMode(info.Mode(), runtime.GOOS) {
		return "", &output.Error{Code: "auth", Message: "unsafe token file permissions", Hint: "chmod 600 " + path}
	}
	if info.Size() > MaxTokenFileSize {
		return "", invalidToken()
	}
	b, e := io.ReadAll(io.LimitReader(f, MaxTokenFileSize+1))
	if e != nil {
		return "", fileError("read token file", path, e)
	}
	if len(b) > MaxTokenFileSize {
		return "", invalidToken()
	}
	return ValidateToken(b)
}
