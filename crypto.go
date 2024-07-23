package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
)

// generateID creates a random ID string.
func generateID() string {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		log.Fatalf("failed to generate ID: %v", err)
	}
	return hex.EncodeToString(buffer)
}

// hashKey generates an MD5 hash of the given key string.
func hashKey(key string) string {
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// newEncryptionKey generates a random 32-byte encryption key.
func newEncryptionKey() []byte {
	keyBuffer := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, keyBuffer); err != nil {
		log.Fatalf("failed to generate encryption key: %v", err)
	}
	return keyBuffer
}

// copyStream performs the encryption/decryption and copies data from src to dst.
func copyStream(stream cipher.Stream, blockSize int, src io.Reader, dst io.Writer) (int, error) {
	buffer := make([]byte, 32*1024)
	totalBytesWritten := 0

	for {
		bytesRead, readErr := src.Read(buffer)
		if bytesRead > 0 {
			stream.XORKeyStream(buffer[:bytesRead], buffer[:bytesRead])
			bytesWritten, writeErr := dst.Write(buffer[:bytesRead])
			if writeErr != nil {
				return totalBytesWritten, fmt.Errorf("failed to write to destination: %w", writeErr)
			}
			totalBytesWritten += bytesWritten
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return totalBytesWritten, fmt.Errorf("failed to read from source: %w", readErr)
		}
	}
	return totalBytesWritten, nil
}

// copyAndDecrypt reads from src, decrypts using key, and writes to dst.
func copyAndDecrypt(encryptionKey []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return 0, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, fmt.Errorf("failed to read IV from source: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	return copyStream(stream, block.BlockSize(), src, dst)
}

// copyAndEncrypt reads from src, encrypts using key, and writes to dst.
func copyAndEncrypt(encryptionKey []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return 0, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, fmt.Errorf("failed to generate IV: %w", err)
	}

	if _, err := dst.Write(iv); err != nil {
		return 0, fmt.Errorf("failed to write IV to destination: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	return copyStream(stream, block.BlockSize(), src, dst)
}
