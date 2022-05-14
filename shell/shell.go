// Package shell provides basic shell interaction capabilities
package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

// Run executes the provided shell command and returns STDOUT / STDERR
func Run(command string) (stdout string, err error) {

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
		/* #nosec G304 */
		outFile, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_WRONLY, 0600)
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

	// Execute command
	err = generateCommand(fields, outBuf).Run()

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
	/* #nosec G204 */
	if len(fields) == 1 {
		cmd = exec.Command(fields[0])
	} else {
		cmd = exec.Command(fields[0], fields[1:]...)
	}

	// Attach STDOUT + STDERR to output buffer
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	return
}
