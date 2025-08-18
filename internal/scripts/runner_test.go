package scripts

import (
	"os"
	"strings"
	"testing"
)

// TestCheckShebang tests the CheckShebang function with various scenarios.
func TestCheckShebang(t *testing.T) {
	// Helper function to create a temporary file with given content
	createTempFile := func(t *testing.T, content string) string {
		t.Helper()
		file, err := os.CreateTemp("", "testfile")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer file.Close()
		if _, err := file.WriteString(content); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		return file.Name()
	}

	tests := []struct {
		name     string
		content  string
		filePath string
		want     []string
	}{
		{
			name:     "Valid shebang with single interpreter",
			content:  "#!/bin/bash\nscript content",
			filePath: "test.sh",
			want:     []string{"/bin/bash", "test.sh"},
		},
		{
			name:     "Valid shebang with interpreter and argument",
			content:  "#!/usr/bin/env python3\nscript content",
			filePath: "test.py",
			want:     []string{"/usr/bin/env", "python3 test.py"},
		},
		{
			name:     "No shebang",
			content:  "script content\nno shebang",
			filePath: "test.sh",
			want:     []string{"/bin/sh", "test.sh"},
		},
		{
			name:     "Empty file",
			content:  "",
			filePath: "empty.sh",
			want:     []string{"/bin/sh", "empty.sh"},
		},
		{
			name:     "Shebang with trailing space",
			content:  "#! /bin/sh \nscript content",
			filePath: "test.sh",
			want:     []string{"/bin/sh", "test.sh"},
		},
		{
			name:     "Shebang with multiple arguments",
			content:  "#!/usr/bin/env python3 --verbose\nscript content",
			filePath: "test.py",
			want:     []string{"/usr/bin/env", "python3 --verbose test.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the test content
			filePath := createTempFile(t, tt.content)
			defer os.Remove(filePath)

			// Update filePath in want if it's used
			for i, v := range tt.want {
				if strings.Contains(v, tt.filePath) {
					tt.want[i] = strings.Replace(v, tt.filePath, filePath, 1)
				}
			}

			// Call the function
			got := CheckShebang(filePath)

			// Compare results
			if len(got) != len(tt.want) {
				t.Errorf("CheckShebang() returned %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("CheckShebang() = %v, want %v", got, tt.want)
					return
				}
			}
		})
	}

	// Test case for non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		filePath := "/non/existent/path"
		want := []string{"/bin/sh", filePath}
		got := CheckShebang(filePath)
		if len(got) != len(want) {
			t.Errorf("CheckShebang() returned %v, want %v", got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("CheckShebang() = %v, want %v", got, want)
				return
			}
		}
	})
}
