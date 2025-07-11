package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func urlEncodeInfoHash(hash [20]byte) string {
	var buf bytes.Buffer
	for _, b := range hash {
		// Escape every byte
		buf.WriteString(fmt.Sprintf("%%%02X", b))
	}
	return buf.String()
}

func requestPeersUDP(torrent TorrentFile, client Client) ([]byte, error) {
	parsedURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker URL: %w", err)
	}

	conn, err := net.Dial("udp", parsedURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to UDP tracker: %w", err)
	}
	defer conn.Close()

	// Connection request
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	transactionID := make([]byte, 4)
	rand.Read(transactionID)

	var connReq bytes.Buffer
	binary.Write(&connReq, binary.BigEndian, uint64(0x41727101980)) // connection_id
	binary.Write(&connReq, binary.BigEndian, uint32(0))             // action
	connReq.Write(transactionID)

	_, err = conn.Write(connReq.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to send UDP connection request: %w", err)
	}

	connResp := make([]byte, 16)
	_, err = conn.Read(connResp)
	if err != nil {
		return nil, fmt.Errorf("failed to read UDP connection response: %w", err)
	}

	if !bytes.Equal(transactionID, connResp[4:8]) {
		return nil, fmt.Errorf("UDP transaction ID mismatch")
	}
	connectionID := connResp[8:16]

	// Announce request
	var announceReq bytes.Buffer
	announceReq.Write(connectionID)
	binary.Write(&announceReq, binary.BigEndian, uint32(1)) // action
	announceReq.Write(transactionID)
	announceReq.Write(torrent.InfoHash[:])
	announceReq.Write([]byte(client.PeerID))
	binary.Write(&announceReq, binary.BigEndian, uint64(0)) // downloaded
	binary.Write(&announceReq, binary.BigEndian, uint64(torrent.Info.Length))
	binary.Write(&announceReq, binary.BigEndian, uint64(0)) // uploaded
	binary.Write(&announceReq, binary.BigEndian, uint32(2)) // event
	binary.Write(&announceReq, binary.BigEndian, uint32(0)) // ip address
	binary.Write(&announceReq, binary.BigEndian, uint32(0)) // key
	binary.Write(&announceReq, binary.BigEndian, int32(-1)) // num want
	binary.Write(&announceReq, binary.BigEndian, uint16(client.Port))

	_, err = conn.Write(announceReq.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to send UDP announce request: %w", err)
	}

	announceResp := make([]byte, 1024)
	n, err := conn.Read(announceResp)
	if err != nil {
		return nil, fmt.Errorf("failed to read UDP announce response: %w", err)
	}

	return announceResp[:n], nil
}

func RequestPeers(torrent TorrentFile, client Client) ([]byte, error) {
	parsedURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker URL: %w", err)
	}

	if parsedURL.Scheme == "udp" {
		return requestPeersUDP(torrent, client)
	}

	params := url.Values{}
	params.Set("info_hash", urlEncodeInfoHash(torrent.InfoHash))
	params.Set("peer_id", client.PeerID)
	params.Set("port", strconv.Itoa(client.Port))
	params.Set("uploaded", "0")
	params.Set("downloaded", "0")
	params.Set("left", strconv.Itoa(torrent.Info.Length))
	params.Set("compact", "1")
	params.Set("event", "started")

	trackerURL := torrent.Announce + "?" + params.Encode()

	resp, err := http.Get(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request to tracker failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracker response: %w", err)
	}

	return body, nil
}
