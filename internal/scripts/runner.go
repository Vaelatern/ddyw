package scripts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/xerrors"
)

// Cache for HTTP-fetched scripts (in-memory for simplicity, could be persisted).
var scriptCache = make(map[string][]byte)

// RunViaJson executes a script with JSON input/output in a temporary ramdisk, resolving the script
// from --exec-dir, an HTTP endpoint, or an embedded fs.FS, in that order.
func RunViaJson[Args any, In any, Out any](ctx context.Context, args Args, input In, script string, execDir string, embeddedFS fs.FS, httpEndpoint string) (Out, error) {
	var result Out

	// Create a temporary ramdisk directory
	ramDisk, err := os.MkdirTemp("", "ramdisk-")
	if err != nil {
		return result, xerrors.Errorf("failed to create ramdisk: %w", err)
	}
	defer os.RemoveAll(ramDisk) // Clean up after execution

	// Resolve script path
	scriptPath, err := resolveScript(script, execDir, embeddedFS, httpEndpoint)
	if err != nil {
		return result, xerrors.Errorf("failed to resolve script %s: %w", script, err)
	}

	// Copy script to ramdisk
	scriptDest := filepath.Join(ramDisk, filepath.Base(script))
	if err := copyScript(scriptPath, scriptDest); err != nil {
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

// resolveScript finds the script in --exec-dir, HTTP endpoint, or embedded FS, in that order.
func resolveScript(script, execDir string, embeddedFS fs.FS, httpEndpoint string) (string, error) {
	// 1. Check --exec-dir (defaults to ./exec/)
	if execDir == "" {
		execDir = "./exec/"
	}
	scriptPath := filepath.Join(execDir, script)
	if _, err := os.Stat(scriptPath); err == nil {
		return scriptPath, nil
	}

	// 2. Check HTTP endpoint with caching
	if httpEndpoint != "" {
		cacheKey := httpEndpoint + "/" + script
		if cached, exists := scriptCache[cacheKey]; exists {
			tmpFile, err := os.CreateTemp("", "script-")
			if err != nil {
				return "", xerrors.Errorf("failed to create temp file for cached script: %w", err)
			}
			if _, err := tmpFile.Write(cached); err != nil {
				tmpFile.Close()
				return "", xerrors.Errorf("failed to write cached script: %w", err)
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}

		resp, err := http.Get(httpEndpoint + "/" + script)
		if err == nil && resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return "", xerrors.Errorf("failed to read HTTP response: %w", err)
			}
			scriptCache[cacheKey] = data // Cache the result
			tmpFile, err := os.CreateTemp("", "script-")
			if err != nil {
				return "", xerrors.Errorf("failed to create temp file for HTTP script: %w", err)
			}
			if _, err := tmpFile.Write(data); err != nil {
				tmpFile.Close()
				return "", xerrors.Errorf("failed to write HTTP script: %w", err)
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}

	// 3. Check embedded FS
	if embeddedFS != nil {
		data, err := fs.ReadFile(embeddedFS, script)
		if err == nil {
			tmpFile, err := os.CreateTemp("", "script-")
			if err != nil {
				return "", xerrors.Errorf("failed to create temp file for embedded script: %w", err)
			}
			if _, err := tmpFile.Write(data); err != nil {
				tmpFile.Close()
				return "", xerrors.Errorf("failed to write embedded script: %w", err)
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}

	// 4. Return error if script not found
	return "", xerrors.Errorf("script %s not found in exec-dir, HTTP endpoint, or embedded FS", script)
}

// copyScript copies a script file from src to dest.
func copyScript(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return xerrors.Errorf("failed to open source script: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return xerrors.Errorf("failed to create destination script: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return xerrors.Errorf("failed to copy script: %w", err)
	}
	return nil
}
