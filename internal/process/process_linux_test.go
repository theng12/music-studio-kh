package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("TUNEHARBOR_PROCESS_PARENT") == "1" {
		child := exec.CommandContext(context.Background(), "sleep", "60")
		Configure(child)
		if e := child.Start(); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(2)
		}
		fmt.Println(child.Process.Pid)
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// If the server is killed outright, download and conversion tools it started
// must not keep running.
func TestChildDiesWithKilledParent(t *testing.T) {
	parent := exec.Command(os.Args[0], "-test.run=^$")
	parent.Env = append(os.Environ(), "TUNEHARBOR_PROCESS_PARENT=1")
	out, _ := parent.StdoutPipe()
	if e := parent.Start(); e != nil {
		t.Fatal(e)
	}
	line, e := bufio.NewReader(out).ReadString('\n')
	if e != nil {
		t.Fatal(e)
	}
	pid, _ := strconv.Atoi(line[:len(line)-1])
	parent.Process.Kill()
	parent.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var status syscall.WaitStatus
		if syscall.Kill(pid, 0) != nil {
			return
		}
		// The orphan is reparented; reap it if this test became its subreaper.
		syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		time.Sleep(20 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Fatal("child outlived its killed parent")
}
