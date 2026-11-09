package output

import "unicode"

// displayWidth covers common CJK, Hangul, fullwidth and emoji ranges locally.
// Combining marks and variation selectors occupy no additional terminal cells.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == 0x200d:
		case r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
			(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
			(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
			(r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3fffd)):
			n += 2
		default:
			n++
		}
	}
	return n
}
