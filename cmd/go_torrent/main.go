package main

import (
	"crypto/rand"
	"go_torrent/pkg"
	"log"
)

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
	client := peers.Client{
		PeerID: "",   // Example Peer ID
		Port:   6881, // Example port
	}
	client.PeerID = generatePeerID()

	torrentPath := "big-buck-bunny.torrent"
	torrent, err := peers.ParseTorrentFile(torrentPath)
	if err != nil {
		log.Fatalf("Error parsing torrent: %v", err)
	}

	log.Printf("Tracker URL: %s", torrent.Announce)
	log.Printf("File Name: %s", torrent.Info.Name)
	log.Printf("Piece Length: %d", torrent.Info.PieceLength)
	log.Printf("Total Length: %d", torrent.Info.Length)
	log.Printf("InfoHash: %x", torrent.InfoHash)

	body, err := peers.RequestPeers(torrent, client)
	if err != nil {
		log.Fatalf("Error requesting peers: %v", err)
	}
	log.Printf("Raw tracker response (%d bytes):\n%x", len(body), body)
}

