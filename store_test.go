package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestPathTransformFunc(t *testing.T) {
	key := "momsbestpicture"
	pathKey := CASPathTransformFunc(key)
	expectedFilename := "6804429f74181a63c50c3d81d733a12f14a353ff"
	expectedPathName := "68044/29f74/181a6/3c50c/3d81d/733a1/2f14a/353ff"
	if pathKey.PathName != expectedPathName {
		t.Errorf("PathName: got %s, want %s", pathKey.PathName, expectedPathName)
	}
	if pathKey.Filename != expectedFilename {
		t.Errorf("Filename: got %s, want %s", pathKey.Filename, expectedFilename)
	}
}
func TestStore(t *testing.T) {
	store := newStore()
	id := generateID()
	defer teardown(t, store)
	testCases := []struct {
		key  string
		data []byte
	}{
		{"foo_0", []byte("some jpg bytes")},
		{"foo_1", []byte("different jpg bytes")},
		{"foo_2", []byte("yet another jpg bytes")},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Store and Retrieve key %s", tc.key), func(t *testing.T) {
			// Write file
			if _, err := store.WriteFile(id, tc.key, bytes.NewReader(tc.data)); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}
			time.Sleep(100 * time.Millisecond) // Delay for concurrent access
			// Check existence
			if exists := store.FileExists(id, tc.key); !exists {
				t.Fatalf("FileExists: expected key %s to exist", tc.key)
			}
			// Read file
			_, reader, err := store.ReadFile(id, tc.key)
			if err != nil {
				t.Fatalf("ReadFile error: %v", err)
			}
			readData, _ := io.ReadAll(reader)
			if !bytes.Equal(readData, tc.data) {
				t.Errorf("ReadFile: got %s, want %s", string(readData), string(tc.data))
			}
			// Delete file
			if err := store.DeleteFile(id, tc.key); err != nil {
				t.Fatalf("DeleteFile error: %v", err)
			}
			time.Sleep(100 * time.Millisecond) // Delay for concurrent access
			// Check non-existence
			if exists := store.FileExists(id, tc.key); exists {
				t.Fatalf("FileExists: expected key %s to not exist", tc.key)
			}
		})
	}
}
func TestConcurrentAccess(t *testing.T) {
	store := newStore()
	id := generateID()
	defer teardown(t, store)
	key := "concurrent_key"
	data := []byte("concurrent data bytes")
	// Write file
	if _, err := store.WriteFile(id, key, bytes.NewReader(data)); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	// Concurrent reads
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, reader, err := store.ReadFile(id, key)
			if err != nil {
				t.Errorf("ReadFile error: %v", err)
				return
			}
			readData, _ := io.ReadAll(reader)
			if !bytes.Equal(readData, data) {
				t.Errorf("ReadFile: got %s, want %s", string(readData), string(data))
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// Delete file
	if err := store.DeleteFile(id, key); err != nil {
		t.Fatalf("DeleteFile error: %v", err)
	}
}
func TestEdgeCases(t *testing.T) {
	store := newStore()
	id := generateID()
	defer teardown(t, store)
	// Empty key
	t.Run("Empty Key", func(t *testing.T) {
		key := ""
		data := []byte("data for empty key")
		if _, err := store.WriteFile(id, key, bytes.NewReader(data)); err == nil {
			t.Error("expected WriteFile to fail with empty key")
		}
	})
	// Empty data
	t.Run("Empty Data", func(t *testing.T) {
		key := "empty_data_key"
		data := []byte("")
		if _, err := store.WriteFile(id, key, bytes.NewReader(data)); err != nil {
			t.Errorf("WriteFile error: %v", err)
		}
		if exists := store.FileExists(id, key); !exists {
			t.Errorf("FileExists: expected key %s to exist", key)
		}
		_, reader, err := store.ReadFile(id, key)
		if err != nil {
			t.Errorf("ReadFile error: %v", err)
		}
		readData, _ := io.ReadAll(reader)
		if !bytes.Equal(readData, data) {
			t.Errorf("ReadFile: got %s, want %s", string(readData), string(data))
		}
		if err := store.DeleteFile(id, key); err != nil {
			t.Errorf("DeleteFile error: %v", err)
		}
	})
	// Large data
	t.Run("Large Data", func(t *testing.T) {
		key := "large_data_key"
		data := make([]byte, 10*1024*1024) // 10 MB of data
		if _, err := store.WriteFile(id, key, bytes.NewReader(data)); err != nil {
			t.Errorf("WriteFile error: %v", err)
		}
		if exists := store.FileExists(id, key); !exists {
			t.Errorf("FileExists: expected key %s to exist", key)
		}
		_, reader, err := store.ReadFile(id, key)
		if err != nil {
			t.Errorf("ReadFile error: %v", err)
		}
		readData, _ := io.ReadAll(reader)
		if !bytes.Equal(readData, data) {
			t.Errorf("ReadFile: got %s, want %s", string(readData), string(data))
		}
		if err := store.DeleteFile(id, key); err != nil {
			t.Errorf("DeleteFile error: %v", err)
		}
	})
}
func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	return NewStore(opts)
}
func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}
