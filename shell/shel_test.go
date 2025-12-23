package shell

import "testing"

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
