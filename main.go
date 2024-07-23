package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"DistFileStore/p2p"
)

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)

	// Replace colon in the storage root directory name
	storageRoot := strings.Replace(listenAddr, ":", "", -1) + "_network"

	fileServerOpts := FileServerOpts{
		EncryptionKey:     newEncryptionKey(),
		StorageRoot:       storageRoot,
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	fileServer := NewFileServer(fileServerOpts)
	tcpTransport.OnPeer = fileServer.OnPeer

	return fileServer
}

func testFileOperations(server *FileServer, key string, data string) error {
	dataReader := bytes.NewReader([]byte(data))

	// Store the file
	if err := server.Store(key, dataReader); err != nil {
		return fmt.Errorf("failed to store file %s: %w", key, err)
	}

	// Retrieve the file
	reader, err := server.Get(key)
	if err != nil {
		return fmt.Errorf("failed to get file %s: %w", key, err)
	}

	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", key, err)
	}

	fmt.Printf("Retrieved data for %s: %s\n", key, string(retrievedData))

	// Delete the file
	if err := server.store.DeleteFile(server.ID, key); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", key, err)
	}

	return nil
}

func main() {
	// Create and start servers
	server1 := makeServer(":3000")
	server2 := makeServer(":7000")
	server3 := makeServer(":5000", ":3000", ":7000")

	servers := []*FileServer{server1, server2, server3}

	startServer := func(server *FileServer) {
		if err := server.Start(); err != nil {
			log.Fatalf("failed to start server at %s: %v", server.Transport.Addr(), err)
		}
	}

	go startServer(server1)
	time.Sleep(500 * time.Millisecond)

	go startServer(server2)
	time.Sleep(2 * time.Second)

	go startServer(server3)
	time.Sleep(2 * time.Second)

	// Test file operations
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("file_%d.txt", i)
		data := fmt.Sprintf("This is the content of file %d", i)

		if err := testFileOperations(server3, key, data); err != nil {
			log.Fatalf("error during file operations: %v", err)
		}
	}

	// Test file operations across servers
	for i := 20; i < 40; i++ {
		key := fmt.Sprintf("file_%d.txt", i)
		data := fmt.Sprintf("This is the content of file %d", i)

		for _, server := range servers {
			if err := testFileOperations(server, key, data); err != nil {
				log.Fatalf("error during file operations on server %s: %v", server.Transport.Addr(), err)
			}
		}
	}

	fmt.Println("All tests passed successfully!")
}
