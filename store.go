package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const defaultRootFolderName = "ggnetwork"

// PathKey represents a structured path for stored files.
type PathKey struct {
	PathName string
	Filename string
}

// PathTransformFunc defines a function type for transforming keys into PathKeys.
type PathTransformFunc func(string) PathKey

// StoreOpts holds options for configuring a Store.
type StoreOpts struct {
	Root              string // Root is the folder name of the root, containing all the folders/files of the system
	PathTransformFunc PathTransformFunc
}

// DefaultPathTransformFunc provides a default path transformation function
var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}

// Store represents the file storage system.
type Store struct {
	StoreOpts
}

// NewStore initializes a new Store with the given options.
func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		opts.Root = defaultRootFolderName
	}

	return &Store{
		StoreOpts: opts,
	}
}

// CASPathTransformFunc generates a PathKey using a content-addressable storage (CAS) approach.
func CASPathTransformFunc(fileKey string) PathKey {
	hash := sha1.Sum([]byte(fileKey))
	hashStr := hex.EncodeToString(hash[:])

	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}

	return PathKey{
		PathName: filepath.Join(paths...), // Use filepath.Join for OS-specific paths
		Filename: hashStr,
	}
}

// FirstPathName returns the first directory in the path.
func (p PathKey) FirstPathName() string {
	firstDir := filepath.Dir(p.PathName)
	if firstDir == "." {
		return ""
	}
	// Extract the first directory name
	return filepath.Base(firstDir)
}

// FullPath returns the full path including the filename.
func (p PathKey) FullPath() string {
	return filepath.Join(p.PathName, p.Filename)
}

// FileExists checks if a file exists at the specified path.
func (s *Store) FileExists(fileID string, fileKey string) bool {
	contentAddressPath := s.PathTransformFunc(fileKey)
	fullPath := filepath.Join(s.Root, fileID, contentAddressPath.FullPath())

	_, err := os.Stat(fullPath)
	return !errors.Is(err, os.ErrNotExist)
}

// Clear removes all files in the root directory.
func (s *Store) Clear() error {
	err := os.RemoveAll(s.Root)
	if err != nil {
		return fmt.Errorf("failed to clear the store: %w", err)
	}
	return nil
}

// DeleteFile removes a specific file from the store.
func (s *Store) DeleteFile(fileID, key string) error {
	pathKey := s.PathTransformFunc(key)
	firstPathWithRoot := filepath.Join(s.Root, fileID, pathKey.FirstPathName())

	err := os.RemoveAll(firstPathWithRoot)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("deleted [%s] from disk", pathKey.Filename)
	return nil
}

// WriteFile writes data to a file.
func (s *Store) WriteFile(fileID, key string, dataReader io.Reader) (int64, error) {
	return s.writeToFile(fileID, key, dataReader)
}

// WriteAndDecryptFile writes and decrypts data to a file.
func (s *Store) WriteAndDecryptFile(encryptionKey []byte, fileID, key string, dataReader io.Reader) (int64, error) {
	file, err := s.openFileForWriting(fileID, key)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bytesWritten, err := copyAndDecrypt(encryptionKey, dataReader, file)
	if err != nil {
		return 0, fmt.Errorf("failed to write and decrypt file: %w", err)
	}

	return int64(bytesWritten), nil
}

// openFileForWriting creates the necessary directories and opens a file for writing.
func (s *Store) openFileForWriting(fileID, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	dirPath := filepath.Join(s.Root, fileID, pathKey.PathName)

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	fullFilePath := filepath.Join(s.Root, fileID, pathKey.FullPath())
	file, err := os.Create(fullFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

// writeToFile writes data to a file.
func (s *Store) writeToFile(fileID, key string, dataReader io.Reader) (int64, error) {
	file, err := s.openFileForWriting(fileID, key)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bytesWritten, err := io.Copy(file, dataReader)
	if err != nil {
		return 0, fmt.Errorf("failed to write to file: %w", err)
	}

	return bytesWritten, nil
}

// ReadFile reads data from a file.
func (s *Store) ReadFile(fileID, key string) (int64, io.Reader, error) {
	return s.readFromFile(fileID, key)
}

// readFromFile reads data from a file.
func (s *Store) readFromFile(fileID, key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	fullFilePath := filepath.Join(s.Root, fileID, pathKey.FullPath())

	file, err := os.Open(fullFilePath)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to open file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return 0, nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return fileInfo.Size(), file, nil
}
