package scripts

import (
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

// mockFile implements fs.File and fs.FileInfo for testing.
type mockFile struct {
	reader   io.Reader
	isDir    bool
	statErr  error
	closeErr error
}

func (m *mockFile) Stat() (fs.FileInfo, error) {
	return m, m.statErr
}

func (m *mockFile) Read([]byte) (int, error) {
	return 0, errors.New("not implemented")
}

func (m *mockFile) Close() error {
	return m.closeErr
}

// mockFile also implements fs.FileInfo.
func (m *mockFile) Name() string       { return "testfile" }
func (m *mockFile) Size() int64        { return 0 }
func (m *mockFile) Mode() fs.FileMode  { return 0644 }
func (m *mockFile) ModTime() time.Time { return time.Now() }
func (m *mockFile) IsDir() bool        { return m.isDir }
func (m *mockFile) Sys() interface{}   { return nil }

// mockFS implements fs.FS for error cases.
type mockFS struct {
	openErr error
	file    fs.File
}

func (m *mockFS) Open(name string) (fs.File, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	return m.file, nil
}

func TestResolutionContext_CheckName(t *testing.T) {
	// Test case 1: File does not exist (fsys.Open returns error)
	t.Run("file does not exist", func(t *testing.T) {
		fsys := &mockFS{openErr: errors.New("file not found")}
		ctx := ResolutionContext{}
		result := ctx.CheckName(fsys, "testfile")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	// Test case 2: File exists but stat fails
	t.Run("stat fails", func(t *testing.T) {
		file := &mockFile{statErr: errors.New("stat error"), closeErr: nil}
		fsys := &mockFS{file: file}
		ctx := ResolutionContext{}
		result := ctx.CheckName(fsys, "testfile")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	// Test case 3: File is a directory
	t.Run("file is directory", func(t *testing.T) {
		file := &mockFile{isDir: true, closeErr: nil}
		fsys := &mockFS{file: file}
		ctx := ResolutionContext{}
		result := ctx.CheckName(fsys, "testfile")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	// Test case 4: Valid file
	t.Run("valid file", func(t *testing.T) {
		// Use fstest.MapFS for a valid file
		fsys := fstest.MapFS{
			"testfile": &fstest.MapFile{Mode: 0644}, // Regular file
		}
		ctx := ResolutionContext{}
		result := ctx.CheckName(fsys, "testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		// Clean up
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})
}

func TestResolutionContext_Resolve(t *testing.T) {
	// Test case 1: All filesystems are nil
	t.Run("all filesystems nil", func(t *testing.T) {
		ctx := ResolutionContext{}
		result := ctx.Resolve("testfile")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	// Test case 2: No valid files found
	t.Run("no valid files", func(t *testing.T) {
		fsys := &mockFS{openErr: errors.New("file not found")}
		ctx := ResolutionContext{
			LocalDir:    fsys,
			EmbeddedDir: fsys,
			RemoteDir:   fsys,
		}
		result := ctx.Resolve("testfile")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	// Test case 3: File found in LocalDir with original name
	t.Run("file in LocalDir", func(t *testing.T) {
		fsys := fstest.MapFS{"testfile": &fstest.MapFile{Mode: 0644}}
		ctx := ResolutionContext{
			LocalDir:    fsys,
			EmbeddedDir: &mockFS{openErr: errors.New("not found")},
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})

	// Test case 4: File found in EmbeddedDir with .fallback
	t.Run("file in EmbeddedDir with fallback", func(t *testing.T) {
		fsys := fstest.MapFS{"testfile.fallback": &fstest.MapFile{Mode: 0644}}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})

	// Test case 5: Role-based name adjustment
	t.Run("role-based name adjustment", func(t *testing.T) {
		fsys := fstest.MapFS{"AdminTestfile": &fstest.MapFile{Mode: 0644}}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
			Role:        "Admin",
		}
		result := ctx.Resolve("Testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
		result = ctx.Resolve("testfile")
		if result != nil {
			t.Errorf("expected nil file, got %v", result)
		}
	})

	// Test case 6: Host-based name adjustment
	t.Run("host-based name adjustment", func(t *testing.T) {
		fsys := fstest.MapFS{"server1.testfile": &fstest.MapFile{Mode: 0644}}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
			Host:        "server1",
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})

	// Test case 7: Host and Role-based name adjustment
	t.Run("host and role-based name adjustment", func(t *testing.T) {
		fsys := fstest.MapFS{"server1.Admintestfile": &fstest.MapFile{Mode: 0644}}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
			Host:        "server1",
			Role:        "Admin",
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})

	// Test case 8: Name adjuster order (reverse)
	t.Run("name adjuster order", func(t *testing.T) {
		fsys := fstest.MapFS{
			"server1.Admintestfile": &fstest.MapFile{Mode: 0644},
			"server1.testfile":      &fstest.MapFile{Mode: 0644},
			"Admintestfile":         &fstest.MapFile{Mode: 0644},
			"testfile":              &fstest.MapFile{Mode: 0644},
		}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
			Host:        "server1",
			Role:        "Admin",
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		// Expect server1.Admintestfile to be grabbed
		info, err := result.Stat()
		if err != nil {
			t.Errorf("failed to stat file: %v", err)
		}
		if info.Name() != "server1.Admintestfile" { // Name() typically returns base name
			t.Errorf("expected file with base name 'server1.Admintestfile', got %v", info.Name())
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})

	// Test case 9: Host and Role-based ignored when not present
	t.Run("host and role-based name adjustment", func(t *testing.T) {
		fsys := fstest.MapFS{
			"testfile":       &fstest.MapFile{Mode: 0644},
			"test1.testfile": &fstest.MapFile{Mode: 0644},
		}
		ctx := ResolutionContext{
			LocalDir:    &mockFS{openErr: errors.New("not found")},
			EmbeddedDir: fsys,
			RemoteDir:   &mockFS{openErr: errors.New("not found")},
			Host:        "server1",
			Role:        "Admin",
		}
		result := ctx.Resolve("testfile")
		if result == nil {
			t.Error("expected non-nil file, got nil")
		}
		// Expect testfile to be grabbed
		info, err := result.Stat()
		if err != nil {
			t.Errorf("failed to stat file: %v", err)
		}
		if info.Name() != "testfile" { // Name() typically returns base name
			t.Errorf("expected file with base name 'server1.Admintestfile', got %v", info.Name())
		}
		if result != nil {
			if err := result.Close(); err != nil {
				t.Errorf("failed to close file: %v", err)
			}
		}
	})
}
