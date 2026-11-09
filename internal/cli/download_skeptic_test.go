package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSkepticFilenames(t *testing.T) {
	for _, name := range []string{"CON", "prn", "AUX", "NUL", "COM0", "COM9", "LPT0", "LPT9", "COM¹", "LPT²", "LPT³", "CONIN$", "conout$"} {
		for _, ext := range []string{"", ".pdf"} {
			if got := safeFilename(name + ext); got != "_"+name+ext {
				t.Errorf("%q: %q", name+ext, got)
			}
		}
	}
	for name, want := range map[string]string{"~": "_~", "abc\u202efdp.exe": "abcfdp.exe", "a\u200db": "ab"} {
		if got := safeFilename(name); got != want {
			t.Errorf("%q: %q", name, got)
		}
	}
}

func TestDeviceDownloadsCreateFiles(t *testing.T) {
	isolate(t)
	var filename string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		_, _ = w.Write([]byte("file bytes"))
	}))
	defer srv.Close()
	token := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(token, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SLOWBOOKS_URL", srv.URL)
	t.Setenv("SLOWBOOKS_TOKEN_FILE", token)
	t.Chdir(t.TempDir())
	for _, filename = range []string{"CONOUT$", "CONIN$", "COM0", "LPT0", "COM¹", "LPT²", "LPT³"} {
		code, out, _ := invoke(t, "", "download", "/api/file")
		if code != 0 {
			t.Fatal(code, out)
		}
		b, e := os.ReadFile("_" + filename)
		if e != nil || string(b) != "file bytes" {
			t.Fatal(filename, string(b), e)
		}
	}
}
