package config

import (
	"strings"
	"testing"
)

func TestTokenLengthRoundTrip(t *testing.T) {
	isolate(t)
	token := "sbp_" + strings.Repeat("x", 4092)
	if e := SaveLogin("boundary", Profile{URL: "https://example.test"}, token); e != nil {
		t.Fatal(e)
	}
	st, _ := Load()
	got, e := TokenFile(st.Profiles["boundary"].TokenFile)
	if e != nil || got != token {
		t.Fatal("accepted token failed round trip", e)
	}
	if e = SaveLogin("overlong", Profile{URL: "https://example.test"}, token+"x"); e == nil {
		t.Fatal("overlong token accepted")
	}
}
