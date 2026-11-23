package daemon

import (
	"fmt"
	"io"
	"os"
	"testing"
)

// TestHelperProcess is the "background" program Spawn launches in TestSpawn.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("DAEMON_TEST_HELPER") != "1" {
		return
	}
	Init()
	key, _ := io.ReadAll(os.Stdin)
	fmt.Printf("key=%s\n", key)
	os.Exit(0)
}

func TestSpawn(t *testing.T) {
	t.Setenv("DAEMON_TEST_HELPER", "1")
	child, err := Spawn([]byte("secret"), "-test.run=TestHelperProcess")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for l := range child.Lines {
		got = append(got, l)
	}
	if len(got) == 0 || got[0] != "key=secret" {
		t.Fatalf("child output = %v", got)
	}
}

func TestRunning(t *testing.T) {
	dir := t.TempDir()
	if _, ok := Running(dir); ok {
		t.Fatal("no lock file, should not be running")
	}
	if err := Acquire(dir); err != nil {
		t.Fatal(err)
	}
	if pid, ok := Running(dir); !ok || pid != os.Getpid() {
		t.Fatalf("Running = %d, %v", pid, ok)
	}
	if err := Acquire(dir); err == nil {
		t.Fatal("a second Acquire must fail while the first one is held")
	}
	Release(dir)
	if _, ok := Running(dir); ok {
		t.Fatal("after Release it must not be running")
	}
}
