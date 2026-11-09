package config

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentLoginsPreserveProfilesAndCredentials(t *testing.T) {
	isolate(t)
	run := func(t *testing.T, shared bool) {
		var wg sync.WaitGroup
		start := make(chan struct{})
		failures := make(chan error, 32)
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				name := fmt.Sprintf("p%d", i)
				if shared {
					name = "shared"
				}
				role := fmt.Sprintf("r%d", i)
				// On slow disks, 32 serialized fsyncs may exceed the public two-second
				// lock budget. Retry only a refused lock acquisition, never a mutation.
				deadline := time.Now().Add(30 * time.Second)
				for {
					err := SaveLogin(name, Profile{URL: "https://example.test", Role: role}, "sbp_"+role)
					if err == nil || !strings.HasPrefix(err.Error(), "config is busy: ") || time.Now().After(deadline) {
						failures <- err
						return
					}
				}
			}(i)
		}
		close(start)
		wg.Wait()
		close(failures)
		for e := range failures {
			if e != nil {
				t.Fatal(e)
			}
		}
		st, e := Load()
		if e != nil {
			t.Fatal(e)
		}
		if !shared && len(st.Profiles) != 32 {
			t.Fatalf("lost profiles: got %d, want 32", len(st.Profiles))
		}
		for _, p := range st.Profiles {
			token, e := TokenFile(p.TokenFile)
			if e != nil || token != "sbp_"+p.Role {
				t.Fatal("stored role and credential diverged", e)
			}
		}
	}
	t.Run("distinct", func(t *testing.T) { run(t, false) })
	t.Run("shared", func(t *testing.T) { run(t, true) })
}
