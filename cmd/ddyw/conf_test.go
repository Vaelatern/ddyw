package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"
)

// mockFile is a custom implementation of fs.File for testing.
type mockFile struct {
	reader io.Reader
	name   string
}

func (m *mockFile) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *mockFile) Stat() (fs.FileInfo, error) {
	return &mockFileInfo{name: m.name}, nil
}

func (m *mockFile) Close() error {
	return nil
}

type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock read error")
}

// TestDeepMergeFiles tests the DeepMergeFiles function with various scenarios.
func TestDeepMergeFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      []fs.File
		wantResult map[string]interface{}
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "Successful TOML merge",
			files: []fs.File{
				&mockFile{
					reader: bytes.NewBufferString(`key1 = "value1"`),
					name:   "file1.toml",
				},
				&mockFile{
					reader: bytes.NewBufferString(`key2 = "value2"`),
					name:   "file2.toml",
				},
			},
			wantResult: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "Successful YAML merge",
			files: []fs.File{
				&mockFile{
					reader: bytes.NewBufferString(`key1: value1`),
					name:   "file1.yaml",
				},
				&mockFile{
					reader: bytes.NewBufferString(`key2: value2`),
					name:   "file2.yaml",
				},
			},
			wantResult: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "Mixed TOML and YAML merge",
			files: []fs.File{
				&mockFile{
					reader: bytes.NewBufferString(`key1 = "value1"`),
					name:   "file1.toml",
				},
				&mockFile{
					reader: bytes.NewBufferString(`key2: value2`),
					name:   "file2.yaml",
				},
			},
			wantResult: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "Read file error",
			files: []fs.File{
				&mockFile{
					reader: &errorReader{},
					name:   "file1.toml",
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to read file file1.toml",
		},
		{
			name: "Invalid TOML and YAML",
			files: []fs.File{
				&mockFile{
					reader: bytes.NewBufferString(`invalid content`),
					name:   "file1.txt",
				},
			},
			wantErr:    true,
			wantErrMsg: "file file1.txt is neither valid TOML nor YAML",
		},
		{
			name:       "Empty file list",
			files:      []fs.File{},
			wantResult: map[string]interface{}{}, // Expect empty map
			wantErr:    false,
		},
		{
			name: "Deep merge with nested maps",
			files: []fs.File{
				&mockFile{
					reader: bytes.NewBufferString(`
                        [section]
                        key1 = "value1"
                        [section.subsection]
                        key2 = "value2"
                    `),
					name: "file1.toml",
				},
				&mockFile{
					reader: bytes.NewBufferString(`
                        section:
                            key3: value3
                            subsection:
                                key4: value4
                    `),
					name: "file2.yaml",
				},
			},
			wantResult: map[string]interface{}{
				"section": map[string]interface{}{
					"key1": "value1",
					"key3": "value3",
					"subsection": map[string]interface{}{
						"key2": "value2",
						"key4": "value4",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeepMergeFiles(tt.files)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeepMergeFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("DeepMergeFiles() error = %v, want error containing %q", err, tt.wantErrMsg)
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.wantResult) {
					t.Errorf("DeepMergeFiles() got = %v, want %v", got, tt.wantResult)
				}
			}
			// Clean up file resources
			for _, f := range tt.files {
				f.Close()
			}
		})
	}
}

// TestDeepMergeMaps tests the deepMergeMaps function with various scenarios.
func TestDeepMergeMaps(t *testing.T) {
	tests := []struct {
		name   string
		source map[string]interface{}
		target map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name:   "Simple merge",
			source: map[string]interface{}{"key1": "value1"},
			target: map[string]interface{}{"key2": "value2"},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:   "Source overrides target",
			source: map[string]interface{}{"key1": "new"},
			target: map[string]interface{}{"key1": "old", "key2": "value2"},
			want: map[string]interface{}{
				"key1": "new",
				"key2": "value2",
			},
		},
		{
			name: "Nested map merge",
			source: map[string]interface{}{
				"section": map[string]interface{}{
					"key1": "value1",
					"key3": "value3",
				},
			},
			target: map[string]interface{}{
				"section": map[string]interface{}{
					"key1": "old",
					"key2": "value2",
				},
			},
			want: map[string]interface{}{
				"section": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
		},
		{
			name:   "Nil source",
			source: nil,
			target: map[string]interface{}{"key1": "value1"},
			want:   map[string]interface{}{"key1": "value1"},
		},
		{
			name:   "Empty source and target",
			source: map[string]interface{}{},
			target: map[string]interface{}{},
			want:   map[string]interface{}{},
		},
		{
			name: "Non-map value in source",
			source: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			target: map[string]interface{}{
				"key2": map[string]interface{}{"subkey": "subvalue"},
			},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepMergeMaps(tt.source, tt.target)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deepMergeMaps() got = %v, want %v", got, tt.want)
			}
		})
	}
}
