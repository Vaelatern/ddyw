package scripts

import (
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

	"github.com/leonklingele/securetemp"
)

// Cache for HTTP-fetched scripts (in-memory for simplicity, could be persisted).
var scriptCache = make(map[string][]byte)

// RunViaJson executes a script with JSON input/output in a temporary ramdisk
func RunViaJson[Out any, Args any, In any](ctx context.Context, args Args, input In, scriptFile fs.File) (Out, error) {
	var result Out
	finalStat, err := scriptFile.Stat()
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
	cmd := exec.CommandContext(ctx, "/bin/sh", finalPath)
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
