// Package shell provides basic shell interaction capabilities
package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
)

// Options represents options for running shell commands
type Options struct {
	StdInData []byte
}

// Run executes the provided shell command and returns STDOUT / STDERR
func Run(command string) (stdout string, err error) {
	return RunWithOptions(command, nil)
}

// RunWithOptions executes the provided shell command and returns STDOUT / STDERR
func RunWithOptions(command string, opts *Options) (stdout string, err error) {

	if command == "" {
		return
	}

	var (
		outStringBuf bytes.Buffer
		outBuf       io.Writer = &outStringBuf
	)

	// Check if the command requests a redirect of STDOUT / STDERR to file
	command, outFilePath, err := splitByRedirect(command)
	if err != nil {
		return "", err
	}
	if outFilePath != "" {
		outFile, err := os.OpenFile(filepath.Clean(outFilePath), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		defer func() {
			cerr := outFile.Close()
			if err == nil {
				err = cerr
			}
		}()

		outBuf = bufio.NewWriter(outFile)
	}

	// Parse command line into command + arguments
	fields, err := shlex.Split(command)
	if err != nil || len(fields) == 0 {
		return "", fmt.Errorf("failed to parse command (%s): %w", command, err)
	}

	// Generate command
	cmd := generateCommand(fields, outBuf)

	// Handle options
	if opts != nil {

		// Attach STDIN data if provided
		if opts.StdInData != nil {
			cmd.Stdin = bytes.NewReader(opts.StdInData)
		}
	}

	// Execute command
	err = cmd.Run()

	return outStringBuf.String(), err
}

func splitByRedirect(command string) (string, string, error) {

	split := strings.Split(command, ">")

	if len(split) == 1 {
		return command, "", nil
	}

	if len(split) == 2 {

		outFields, err := shlex.Split(split[1])
		if err != nil {
			return "", "", fmt.Errorf("failed to parse output file path: %w", err)
		}
		if len(outFields) != 1 {
			return "", "", fmt.Errorf("invalid syntax: %s", command)
		}

		return split[0], outFields[0], nil
	}

	return "", "", fmt.Errorf("invalid syntax: %s", command)
}

func generateCommand(fields []string, outBuf io.Writer) (cmd *exec.Cmd) {

	// Check if any arguments were provided
	if len(fields) == 1 {
		cmd = exec.Command(fields[0]) // #nosec G204
	} else {
		cmd = exec.Command(fields[0], fields[1:]...) // #nosec G204
	}

	// Attach STDOUT + STDERR to output buffer
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	return
}
