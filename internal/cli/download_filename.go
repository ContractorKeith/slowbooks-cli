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
		if unicode.IsControl(r) {
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
	if name == "-" {
		return "_-"
	}
	name = limitFilename(name)
	stem, _, _ := strings.Cut(name, ".")
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
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
