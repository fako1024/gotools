package shell

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

const input = `Hello world
This is a test`

func TestSimpleCommand(t *testing.T) {
	cmd := "echo '" + input + "'"
	stdout, err := Run(cmd)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stdout != input+"\n" {
		t.Fatalf("Unexpected output: got %q, want %q", stdout, input+"\n")
	}
}

func TestPipes(t *testing.T) {
	cmd := "cat"
	stdout, err := RunWithOptions(cmd, &Options{
		StdInData: []byte(input),
	})
	if err != nil {
		t.Fatalf("RunWithOptions failed: %v", err)
	}

	if stdout != input {
		t.Fatalf("Unexpected output: got %q, want %q", stdout, input)
	}
}

func TestCreateSession(t *testing.T) {
	// Use PGID (process group ID) which is available on Darwin and Linux
	parentPid := os.Getpid()
	parentPgidOut, err := Run("ps -o pgid= -p " + strconv.Itoa(parentPid))
	if err != nil {
		t.Fatalf("Failed to get parent process group ID: %v", err)
	}
	parentPGID, err := strconv.Atoi(strings.TrimSpace(parentPgidOut))
	if err != nil {
		t.Fatalf("Failed to parse parent PGID from output %q: %v", parentPgidOut, err)
	}

	// Helper to get child PID and PGID in one go
	childInfoCmd := "sh -c 'echo PID=$$; echo PGID=$(ps -o pgid= -p $$)'"

	// Test with CreateSession: true - should create a new session/group
	t.Run("WithCreateSession", func(t *testing.T) {
		stdout, err := RunWithOptions(childInfoCmd, &Options{CreateSession: true})
		if err != nil {
			t.Fatalf("RunWithOptions with CreateSession failed: %v", err)
		}
		var childPID, childPGID int
		for _, ln := range strings.Split(strings.TrimSpace(stdout), "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "PID=") {
				childPID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PID from line %q: %v", ln, err)
				}
			} else if strings.HasPrefix(ln, "PGID=") {
				childPGID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PGID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PGID from line %q: %v", ln, err)
				}
			}
		}
		if childPID == 0 || childPGID == 0 {
			t.Fatalf("Failed to obtain child PID/PGID from output: %q", stdout)
		}
		// After setsid, the process becomes session and process group leader: PGID == PID
		if childPGID != childPID {
			t.Errorf("Expected child PGID==PID in new session, got PGID=%d PID=%d", childPGID, childPID)
		}
		if childPGID == parentPGID {
			t.Errorf("Expected child PGID to differ from parent PGID in new session: parentPGID=%d childPGID=%d", parentPGID, childPGID)
		}
	})

	// Test without CreateSession - should keep the same group/session as parent (PGID equal to parentPGID)
	t.Run("WithoutCreateSession", func(t *testing.T) {
		stdout, err := RunWithOptions(childInfoCmd, nil)
		if err != nil {
			t.Fatalf("RunWithOptions without CreateSession failed: %v", err)
		}
		var childPID, childPGID int
		for _, ln := range strings.Split(strings.TrimSpace(stdout), "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "PID=") {
				childPID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PID from line %q: %v", ln, err)
				}
			} else if strings.HasPrefix(ln, "PGID=") {
				childPGID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PGID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PGID from line %q: %v", ln, err)
				}
			}
		}
		if childPID == 0 || childPGID == 0 {
			t.Fatalf("Failed to obtain child PID/PGID from output: %q", stdout)
		}
		if childPGID != parentPGID {
			t.Errorf("Expected child PGID to equal parent PGID without CreateSession: parentPGID=%d childPGID=%d", parentPGID, childPGID)
		}
	})

	// Test with CreateSession: false explicitly - equivalent to default
	t.Run("WithCreateSessionFalse", func(t *testing.T) {
		stdout, err := RunWithOptions(childInfoCmd, &Options{CreateSession: false})
		if err != nil {
			t.Fatalf("RunWithOptions with CreateSession=false failed: %v", err)
		}
		var childPID, childPGID int
		for _, ln := range strings.Split(strings.TrimSpace(stdout), "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "PID=") {
				childPID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PID from line %q: %v", ln, err)
				}
			} else if strings.HasPrefix(ln, "PGID=") {
				childPGID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "PGID=")))
				if err != nil {
					t.Fatalf("Failed to parse child PGID from line %q: %v", ln, err)
				}
			}
		}
		if childPID == 0 || childPGID == 0 {
			t.Fatalf("Failed to obtain child PID/PGID from output: %q", stdout)
		}
		if childPGID != parentPGID {
			t.Errorf("Expected child PGID to equal parent PGID with CreateSession=false: parentPGID=%d childPGID=%d", parentPGID, childPGID)
		}
	})
}
