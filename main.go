package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	bencode "github.com/jackpal/bencode-go"
	"log"
	"os"
)

type Info struct {
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Length      int    `bencode:"length"`
}

type TorrentFile struct {
	Announce string   `bencode:"announce"`
	Info     Info     `bencode:"info"`
	InfoRaw  []byte   `bencode:"-"`
	InfoHash [20]byte `bencode:"-"`
}

type Client struct {
	PeerID string
	Port   int
}

func EncodeRawInfo(info map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, info)
	return buf.Bytes(), err
}

func ComputeInfoHash(data []byte) [20]byte {
	return sha1.Sum(data)
}

func ParseTorrentFile(path string) (TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return TorrentFile{}, err
	}
	defer file.Close()

	data, err := bencode.Decode(file)
	if err != nil {
		return TorrentFile{}, err
	}

	result, ok := data.(map[string]any)
	if !ok {
		return TorrentFile{}, errors.New("invalid torrent format")
	}

	var torrent TorrentFile
	if announce, ok := result["announce"].(string); ok {
		torrent.Announce = announce
	}

	info, ok := result["info"].(map[string]any)
	if !ok {
		return TorrentFile{}, errors.New("missing or invalid 'info' section")
	}

	// Compute InfoRaw
	rawInfo, err := EncodeRawInfo(info)
	if err != nil {
		return TorrentFile{}, err
	}
	torrent.InfoRaw = rawInfo
	torrent.InfoHash = ComputeInfoHash(rawInfo)

	// Decode info into typed struct
	var parsed Info
	err = bencode.Unmarshal(bytes.NewReader(rawInfo), &parsed)
	if err != nil {
		return TorrentFile{}, err
	}
	torrent.Info = parsed

	return torrent, nil
}

func generatePeerID() string {
	prefix := "-GC0001-"
	suffix := make([]byte, 12)
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	for i := range suffix {
		b := make([]byte, 1)
		rand.Read(b)
		suffix[i] = charset[int(b[0])%len(charset)]
	}
	return prefix + string(suffix)
}

func main() {
	client := Client{
		PeerID: "",   // Example Peer ID
		Port:   6881, // Example port
	}
	client.PeerID = generatePeerID()

	torrentPath := "big-buck-bunny.torrent"
	torrent, err := ParseTorrentFile(torrentPath)
	if err != nil {
		log.Fatalf("Error parsing torrent: %v", err)
	}

	log.Printf("Tracker URL: %s", torrent.Announce)
	log.Printf("File Name: %s", torrent.Info.Name)
	log.Printf("Piece Length: %d", torrent.Info.PieceLength)
	log.Printf("Total Length: %d", torrent.Info.Length)
	log.Printf("InfoHash: %x", torrent.InfoHash)

	body, err := RequestPeers(torrent, client)
	if err != nil {
		log.Fatalf("Error requesting peers: %v", err)
	}
	log.Printf("Raw tracker response (%d bytes):\n%s", len(body), body)
}
