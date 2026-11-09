package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDownloadPortableFilenames(t *testing.T) {
	for input, want := range map[string]string{
		"CONx." + strings.Repeat("a", 251): "_CO." + strings.Repeat("a", 251),
		"C:evil.pdf":                       "C_evil.pdf", "a.pdf:x": "a.pdf_x", "CON.pdf": "_CON.pdf", "NUL": "_NUL", "COM1": "_COM1", "lpt9.txt": "_lpt9.txt",
		`a<>"|?*.pdf`: "a______.pdf", "invoice.pdf. ": "invoice.pdf", "-": "_-", "... ": "",
		strings.Repeat("a", 400) + ".pdf": strings.Repeat("a", 251) + ".pdf",
		strings.Repeat("é", 200) + ".pdf": strings.Repeat("é", 125) + ".pdf",
	} {
		got := safeFilename(input)
		if got != want || len(got) > 255 || !utf8.ValidString(got) {
			t.Errorf("%q: got %q want %q", input, got, want)
		}
	}
}
