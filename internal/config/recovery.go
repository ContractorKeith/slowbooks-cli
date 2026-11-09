package config

import (
	"errors"
	"strings"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// Login alone may set aside malformed state, serialized with other mutations.
func loginStore() (Store, string, error) {
	s, e := Load()
	if e == nil {
		return s, "", nil
	}
	var typed *output.Error
	if !errors.As(e, &typed) || !strings.HasPrefix(typed.Message, "invalid profiles file:") {
		return s, "", e
	}
	release, e := lockStore()
	if e != nil {
		return s, "", e
	}
	defer release()
	s, e = Load()
	if e == nil {
		return s, "", nil
	}
	if !errors.As(e, &typed) || !strings.HasPrefix(typed.Message, "invalid profiles file:") {
		return s, "", e
	}
	path, e := Path()
	if e != nil {
		return s, "", e
	}
	backup := path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	if e = renameFile(path, backup); e != nil {
		return s, "", fileError("back up corrupt profiles", path, e)
	}
	return Store{Profiles: map[string]Profile{}}, "Moved corrupt profiles file to " + backup, nil
}
