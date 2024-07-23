package main

import (
	"DistFileStore/p2p"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type FileServerOpts struct {
	ID                string
	EncryptionKey     []byte
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts
	peerLock sync.Mutex
	peers    map[string]p2p.Peer
	store    *Store
	quitch   chan struct{}
}

// NewFileServer creates a new FileServer with the given options.
func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	if len(opts.ID) == 0 {
		opts.ID = generateID()
	}

	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

// broadcast sends a message to all connected peers.
func (s *FileServer) broadcast(msg *Message) error {
	buffer := new(bytes.Buffer)
	if err := gob.NewEncoder(buffer).Encode(msg); err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	for _, peer := range s.peers {
		if err := peer.Send([]byte{p2p.IncomingMessage}); err != nil {
			return fmt.Errorf("failed to send incoming message indicator: %w", err)
		}
		if err := peer.Send(buffer.Bytes()); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	return nil
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	ID   string
	Key  string
	Size int64
}

type MessageGetFile struct {
	ID  string
	Key string
}

// Get retrieves a file either locally or from the network.
func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.store.FileExists(s.ID, key) {
		fmt.Printf("[%s] serving file (%s) from local disk\n", s.Transport.Addr(), key)
		_, reader, err := s.store.ReadFile(s.ID, key)
		return reader, err
	}

	fmt.Printf("[%s] dont have file (%s) locally, fetching from network...\n", s.Transport.Addr(), key)

	msg := Message{
		Payload: MessageGetFile{
			ID:  s.ID,
			Key: hashKey(key),
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, fmt.Errorf("failed to broadcast message: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	for _, peer := range s.peers {
		// read only the specified number of bytes.
		var fileSize int64
		if err := binary.Read(peer, binary.LittleEndian, &fileSize); err != nil {
			return nil, fmt.Errorf("failed to read file size from peer: %w", err)
		}

		bytesWritten, err := s.store.WriteAndDecryptFile(s.EncryptionKey, s.ID, key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, fmt.Errorf("failed to write decrypted file: %w", err)
		}

		fmt.Printf("[%s] received (%d) bytes over the network from (%s)\n", s.Transport.Addr(), bytesWritten, peer.RemoteAddr())
		peer.CloseStream()
	}

	_, reader, err := s.store.ReadFile(s.ID, key)
	return reader, err
}

// Store saves a file locally and broadcasts the storage message to peers.
func (s *FileServer) Store(key string, reader io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(reader, fileBuffer)
	)

	fileSize, err := s.store.WriteFile(s.ID, key, tee)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	msg := Message{
		Payload: MessageStoreFile{
			ID:   s.ID,
			Key:  hashKey(key),
			Size: fileSize + 16,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return fmt.Errorf("failed to broadcast storage message: %w", err)
	}

	time.Sleep(5 * time.Millisecond)

	var peers []io.Writer
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	multiWriter := io.MultiWriter(peers...)

	if _, err := multiWriter.Write([]byte{p2p.IncomingStream}); err != nil {
		return fmt.Errorf("failed to send incoming stream indicator: %w", err)
	}

	bytesWritten, err := copyAndEncrypt(s.EncryptionKey, fileBuffer, multiWriter)
	if err != nil {
		return fmt.Errorf("failed to encrypt and copy file: %w", err)
	}

	fmt.Printf("[%s] received and written (%d) bytes to disk\n", s.Transport.Addr(), bytesWritten)
	return nil
}

// Stop signals the file server to stop.
func (s *FileServer) Stop() {
	close(s.quitch)
}

// OnPeer handles new peer connections.
func (s *FileServer) OnPeer(peer p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers[peer.RemoteAddr().String()] = peer

	log.Printf("connected with remote %s", peer.RemoteAddr())
	return nil
}

// loop processes incoming RPCs and handles quit signals.
func (s *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to error or user quit action")
		s.Transport.Close()
	}()

	for {
		select {
		case rpc := <-s.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Printf("decoding error: %v", err)
				continue
			}
			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Printf("handle message error: %v", err)
			}

		case <-s.quitch:
			return
		}
	}
}

// handleMessage processes incoming messages based on their type.
func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch payload := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, payload)
	case MessageGetFile:
		return s.handleMessageGetFile(from, payload)
	default:
		return fmt.Errorf("unknown message type: %T", payload)
	}
}

// handleMessageGetFile handles requests for files from peers.
func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.FileExists(msg.ID, msg.Key) {
		return fmt.Errorf("[%s] file (%s) does not exist on disk", s.Transport.Addr(), msg.Key)
	}

	fmt.Printf("[%s] serving file (%s) over the network\n", s.Transport.Addr(), msg.Key)

	fileSize, reader, err := s.store.ReadFile(msg.ID, msg.Key)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	defer reader.(io.ReadCloser).Close()

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not found", from)
	}

	if err := peer.Send([]byte{p2p.IncomingStream}); err != nil {
		return fmt.Errorf("failed to send incoming stream indicator: %w", err)
	}

	if err := binary.Write(peer, binary.LittleEndian, fileSize); err != nil {
		return fmt.Errorf("failed to send file size: %w", err)
	}

	bytesWritten, err := io.Copy(peer, reader)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	fmt.Printf("[%s] sent (%d) bytes over the network to %s\n", s.Transport.Addr(), bytesWritten, from)
	return nil
}

// handleMessageStoreFile handles incoming file storage requests from peers.
func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) not found", from)
	}

	bytesWritten, err := s.store.WriteFile(s.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("[%s] written %d bytes to disk\n", s.Transport.Addr(), bytesWritten)
	peer.CloseStream()
	return nil
}

// bootstrapNetwork connects to bootstrap nodes.
func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		go func(address string) {
			fmt.Printf("[%s] attempting to connect with remote %s\n", s.Transport.Addr(), address)
			if err := s.Transport.Dial(address); err != nil {
				log.Printf("dial error: %v", err)
			}
		}(addr)
	}

	return nil
}

// Start initiates the file server.
func (s *FileServer) Start() error {
	fmt.Printf("[%s] starting file server...\n", s.Transport.Addr())

	if err := s.Transport.ListenAndAccept(); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	if err := s.bootstrapNetwork(); err != nil {
		return fmt.Errorf("failed to bootstrap network: %w", err)
	}

	s.loop()
	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
