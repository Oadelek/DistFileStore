package main

import (
	"bytes"
	"testing"
)

func TestCopyEncryptDecrypt(t *testing.T) {
	payload := "Foo not bar"
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	// Test encryption
	n, err := copyAndEncrypt(key, src, dst)
	if err != nil {
		t.Fatalf("copyAndEncrypt error: %v", err)
	}
	expectedEncryptedLength := 16 + len(payload)
	if n != expectedEncryptedLength {
		t.Errorf("expected encrypted length %d, got %d", expectedEncryptedLength, n)
	}
	// Test decryption
	out := new(bytes.Buffer)
	nw, err := copyAndDecrypt(key, dst, out)
	if err != nil {
		t.Fatalf("copyAndDecrypt error: %v", err)
	}
	if nw != expectedEncryptedLength {
		t.Errorf("expected decrypted length %d, got %d", expectedEncryptedLength, nw)
	}
	if out.String() != payload {
		t.Errorf("expected payload %s, got %s", payload, out.String())
	}
}
func TestEmptyPayload(t *testing.T) {
	payload := ""
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	// Test encryption
	n, err := copyAndEncrypt(key, src, dst)
	if err != nil {
		t.Fatalf("copyAndEncrypt error: %v", err)
	}
	expectedEncryptedLength := 16 // Only IV size
	if n != expectedEncryptedLength {
		t.Errorf("expected encrypted length %d, got %d", expectedEncryptedLength, n)
	}
	// Test decryption
	out := new(bytes.Buffer)
	nw, err := copyAndDecrypt(key, dst, out)
	if err != nil {
		t.Fatalf("copyAndDecrypt error: %v", err)
	}
	if nw != expectedEncryptedLength {
		t.Errorf("expected decrypted length %d, got %d", expectedEncryptedLength, nw)
	}
	if out.String() != payload {
		t.Errorf("expected payload %s, got %s", payload, out.String())
	}
}
func TestLargePayload(t *testing.T) {
	payload := make([]byte, 10*1024*1024) // 10 MB of data
	src := bytes.NewReader(payload)
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	// Test encryption
	n, err := copyAndEncrypt(key, src, dst)
	if err != nil {
		t.Fatalf("copyAndEncrypt error: %v", err)
	}
	expectedEncryptedLength := 16 + len(payload)
	if n != expectedEncryptedLength {
		t.Errorf("expected encrypted length %d, got %d", expectedEncryptedLength, n)
	}
	// Test decryption
	out := new(bytes.Buffer)
	nw, err := copyAndDecrypt(key, dst, out)
	if err != nil {
		t.Fatalf("copyAndDecrypt error: %v", err)
	}
	if nw != expectedEncryptedLength {
		t.Errorf("expected decrypted length %d, got %d", expectedEncryptedLength, nw)
	}
	if !bytes.Equal(out.Bytes(), payload) {
		t.Errorf("decrypted data does not match original payload")
	}
}
func TestInvalidKey(t *testing.T) {
	payload := "Test data"
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	invalidKey := newEncryptionKey() // Different key for decryption
	// Test encryption
	_, err := copyAndEncrypt(key, src, dst)
	if err != nil {
		t.Fatalf("copyAndEncrypt error: %v", err)
	}
	// Test decryption with invalid key
	out := new(bytes.Buffer)
	_, err = copyAndDecrypt(invalidKey, dst, out)
	if err == nil {
		t.Fatalf("expected decryption to fail with invalid key")
	}
}
func TestPartialRead(t *testing.T) {
	payload := "Partial read test"
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	// Test encryption
	_, err := copyAndEncrypt(key, src, dst)
	if err != nil {
		t.Fatalf("copyAndEncrypt error: %v", err)
	}
	// Simulate partial read
	partialDst := bytes.NewReader(dst.Bytes()[:len(dst.Bytes())/2])
	out := new(bytes.Buffer)
	_, err = copyAndDecrypt(key, partialDst, out)
	if err == nil {
		t.Fatalf("expected decryption to fail with partial data")
	}
}
