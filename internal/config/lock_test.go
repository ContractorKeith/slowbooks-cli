package config

import (
	"bufio"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestConfigLockProcessHelper(t *testing.T) {
	if os.Getenv("SBP_CONFIG_LOCK_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	release, e := lockStore()
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	if e = output.New(os.Stdout, os.Stderr).Text("locked\n"); e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Minute)
}
func TestConfigLockBoundsWaitAndRecoversAfterProcessDeath(t *testing.T) {
	isolate(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestConfigLockProcessHelper$")
	cmd.Env = append(os.Environ(), "SBP_CONFIG_LOCK_HELPER=1")
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, e := bufio.NewReader(pipe).ReadString('\n')
	if e != nil || line != "locked\n" {
		t.Fatal(line, e)
	}
	start := time.Now()
	e = SaveLogin("blocked", Profile{URL: "https://example.test"}, "sbp_test")
	if output.ExitCode(e) != 3 || time.Since(start) > 5*time.Second {
		t.Fatal("lock wait not bounded", e)
	}
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	if e = SaveLogin("after-death", Profile{URL: "https://example.test"}, "sbp_test"); e != nil {
		t.Fatal("dead process retained lock", e)
	}
}
