package scripts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leonklingele/securetemp"
)

// Cache for HTTP-fetched scripts (in-memory for simplicity, could be persisted).
var scriptCache = make(map[string][]byte)

// CheckShebang returns a []string after parsing the shebang line.
// In the Linux way, this is split into two: split on the first space.
// If the first line does not begin with #! then the return is []string{"/bin/sh", finalPath}
// We do this because we use /dev/shm to execute stuff, and that's often mounted noexec.
// As such we always need an interpreter to run the program.
func CheckShebang(filePath string) []string {
	// Default return if no valid shebang is found
	result := []string{"/bin/sh", filePath}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return result
	}
	defer file.Close()

	// Create a scanner to read the first line
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return result
	}

	// Get the first line
	firstLine := scanner.Text()

	// Check if it starts with #!
	if !strings.HasPrefix(firstLine, "#!") {
		return result
	}

	// Remove #! and the optional space that POSIX says can be in a shebang line
	command := strings.TrimSpace(strings.TrimPrefix(firstLine, "#!"))

	// Split on first space
	parts := strings.SplitN(command, " ", 2)

	if len(parts) == 1 {
		return []string{parts[0], filePath}
	} else {
		return []string{parts[0], parts[1] + " " + filePath}
	}

	// Return the interpreter and any arguments
	return parts
}

// RunViaJson executes a script with JSON input/output in a temporary ramdisk
func RunViaJson[Out any, Args any, In any](ctx context.Context, args Args, input In, scriptFile fs.File) (Out, error) {
	var result Out
	finalStat, _ := scriptFile.Stat()
	finalName := finalStat.Name()

	// Build a shack to call $HOME
	tmpDir, cleanupDir, err := securetemp.TempDir(10 * securetemp.SizeMB)
	if err != nil {
		return result, fmt.Errorf("failed to create ramdisk: %w", err)
	}
	defer cleanupDir()
	finalPath := filepath.Join(tmpDir, finalName)

	// create new file to write the script to
	outFile, err := os.Create(finalPath)
	if err != nil {
		return result, fmt.Errorf("Can't create a temporary file: %v", err)
	}
	_, err = io.Copy(outFile, scriptFile)
	if err != nil {
		return result, fmt.Errorf("Couldn't copy whole script to temporary destination: %v", err)
	}
	outFile.Close()

	// Prepare JSON input
	wrapped := struct {
		Local  Args
		Passed In
	}{
		Local:  args,
		Passed: input,
	}
	data, err := json.Marshal(wrapped)
	if err != nil {
		return result, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute script in ramdisk
	shebangLine := CheckShebang(finalPath)
	cmd := exec.CommandContext(ctx, shebangLine[0], shebangLine[1])
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)
	cmd.Stdin = bytes.NewReader(data)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("command errored: %s: %w", outBuf.String(), errors.Join(err, fmt.Errorf("stderr: %s", errBuf.String())))
	}

	// Unmarshal output
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		return result, fmt.Errorf("unmarshalling errored: %s: %w", outBuf.String(), errors.Join(err, fmt.Errorf("stderr: %s", errBuf.String())))
	}

	return result, nil
}

// copyScript copies a script file from src to dest.
func copyScript(src fs.File, dest string) error {
	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination script: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, src); err != nil {
		return fmt.Errorf("failed to copy script: %w", err)
	}
	return nil
}
