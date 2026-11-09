package cli

import (
	"strings"
	"testing"
)

func TestLogoutPromotesRemainingProfile(t *testing.T) {
	_, url := setupAuth(t)
	for _, p := range []string{"original", "zebra", "alpha"} {
		code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--profile", p, "--url", url)
		if code != 0 {
			t.Fatal(code, out)
		}
	}
	code, out, _ := invoke(t, "", "auth", "logout", "--profile", "original")
	if code != 0 || !strings.Contains(out, "alpha") {
		t.Fatal("logout did not report replacement default", code, out)
	}
	code, out, _ = invoke(t, "", "auth", "status")
	if code != 0 {
		t.Fatal("remaining profile stranded", code, out)
	}
	code, out, _ = invoke(t, "", "profile", "list")
	if code != 0 || !strings.Contains(out, `"default":"alpha"`) {
		t.Fatal(code, out)
	}
}
