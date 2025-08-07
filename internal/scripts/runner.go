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

	"golang.org/x/xerrors"
)

// Cache for HTTP-fetched scripts (in-memory for simplicity, could be persisted).
var scriptCache = make(map[string][]byte)

// RunViaJson executes a script with JSON input/output in a temporary ramdisk
func RunViaJson[Out any, Args any, In any](ctx context.Context, args Args, input In, resctx ResolutionContext, script string) (Out, error) {
	var result Out

	// Create a temporary ramdisk directory
	ramDisk, err := os.MkdirTemp("", "ramdisk-")
	if err != nil {
		return result, xerrors.Errorf("failed to create ramdisk: %w", err)
	}
	defer os.RemoveAll(ramDisk) // Clean up after execution

	// Resolve script path
	scriptFile := resctx.Resolve(script)
	if scriptFile == nil {
		return result, xerrors.Errorf("failed to resolve script %s: %w", script, err)
	}
	defer scriptFile.Close()

	// Copy script to ramdisk
	scriptDest := filepath.Join(ramDisk, filepath.Base(script))
	if err := copyScript(scriptFile, scriptDest); err != nil {
		return result, xerrors.Errorf("failed to copy script to ramdisk: %w", err)
	}

	// Ensure script is executable
	if err := os.Chmod(scriptDest, 0755); err != nil {
		return result, xerrors.Errorf("failed to set executable permissions: %w", err)
	}

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
		return result, xerrors.Errorf("failed to marshal input: %w", err)
	}

	// Execute script in ramdisk
	cmd := exec.CommandContext(ctx, scriptDest)
	cmd.Stdin = bytes.NewReader(data)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return result, xerrors.Errorf("command errored: %s: %w", outBuf.String(), errors.Join(err, fmt.Errorf("stderr: %s", errBuf.String())))
	}

	// Unmarshal output
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		return result, xerrors.Errorf("unmarshalling errored: %s: %w", outBuf.String(), errors.Join(err, fmt.Errorf("stderr: %s", errBuf.String())))
	}

	return result, nil
}

// copyScript copies a script file from src to dest.
func copyScript(src fs.File, dest string) error {
	destFile, err := os.Create(dest)
	if err != nil {
		return xerrors.Errorf("failed to create destination script: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, src); err != nil {
		return xerrors.Errorf("failed to copy script: %w", err)
	}
	return nil
}
