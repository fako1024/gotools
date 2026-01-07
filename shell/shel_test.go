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
	// Get the current process's session ID using ps
	parentSidOut, err := Run("ps -o sid= -p " + strconv.Itoa(os.Getpid()))
	if err != nil {
		t.Fatalf("Failed to get parent session ID: %v", err)
	}

	parentSid, err := strconv.Atoi(strings.TrimSpace(parentSidOut))
	if err != nil {
		t.Fatalf("Failed to parse parent session ID from output %q: %v", parentSidOut, err)
	}

	// Test with CreateSession: true - should create a new session
	t.Run("WithCreateSession", func(t *testing.T) {
		// Use sh -c to execute a command that prints its session ID
		stdout, err := RunWithOptions("sh -c 'ps -o sid= -p $$'", &Options{
			CreateSession: true,
		})
		if err != nil {
			t.Fatalf("RunWithOptions with CreateSession failed: %v", err)
		}

		childSid, err := strconv.Atoi(strings.TrimSpace(stdout))
		if err != nil {
			t.Fatalf("Failed to parse child session ID from output %q: %v", stdout, err)
		}

		if childSid == parentSid {
			t.Errorf("Expected new session ID, but got same as parent: parent=%d, child=%d", parentSid, childSid)
		}
	})

	// Test without CreateSession - should use the same session
	t.Run("WithoutCreateSession", func(t *testing.T) {
		stdout, err := RunWithOptions("sh -c 'ps -o sid= -p $$'", nil)
		if err != nil {
			t.Fatalf("RunWithOptions without CreateSession failed: %v", err)
		}

		childSid, err := strconv.Atoi(strings.TrimSpace(stdout))
		if err != nil {
			t.Fatalf("Failed to parse child session ID from output %q: %v", stdout, err)
		}

		if childSid != parentSid {
			t.Errorf("Expected same session ID as parent, but got different: parent=%d, child=%d", parentSid, childSid)
		}
	})

	// Test with CreateSession: false explicitly - should use the same session
	t.Run("WithCreateSessionFalse", func(t *testing.T) {
		stdout, err := RunWithOptions("sh -c 'ps -o sid= -p $$'", &Options{
			CreateSession: false,
		})
		if err != nil {
			t.Fatalf("RunWithOptions with CreateSession=false failed: %v", err)
		}

		childSid, err := strconv.Atoi(strings.TrimSpace(stdout))
		if err != nil {
			t.Fatalf("Failed to parse child session ID from output %q: %v", stdout, err)
		}

		if childSid != parentSid {
			t.Errorf("Expected same session ID as parent, but got different: parent=%d, child=%d", parentSid, childSid)
		}
	})
}
