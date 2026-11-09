package cli

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// safeFilename treats both platform separators as untrusted and strips hidden,
// traversal and control sequences from the server-provided basename.
func safeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		if strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "")
	}
	name = strings.TrimLeft(name, ". ")
	if strings.ContainsAny(name, "/\\") {
		return ""
	}
	name = strings.TrimRight(name, ". ")
	if name == "-" || name == "~" {
		return "_" + name
	}
	name = limitFilename(name)
	stem, _, _ := strings.Cut(name, ".")
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	if reservedDevice(stem) {
		name = limitFilename("_" + name)
	}

	return strings.TrimRight(name, ". ")
}

func trimFilenameBytes(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	name = name[:limit]
	for !utf8.ValidString(name) {
		name = name[:len(name)-1]
	}
	return name
}

func limitFilename(name string) string {
	if len(name) > 255 {
		ext := path.Ext(name)
		if len(ext) > 254 {
			ext = ""
		}
		name = trimFilenameBytes(strings.TrimSuffix(name, ext), 255-len(ext)) + ext
	}
	return name
}

func reservedDevice(stem string) bool {
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT") {
		suffix := stem[3:]
		return len([]rune(suffix)) == 1 && strings.ContainsAny(suffix, "0123456789¹²³")
	}
	return false
}
