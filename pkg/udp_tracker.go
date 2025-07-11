package peers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"time"
)

// UDP tracker protocol constants
const (
	connectAction  = 0
	announceAction = 1
	errorAction    = 3
)

const magicConnectionID = 0x41727101980

type udpConnectionReq struct {
	ConnectionID  uint64
	Action        uint32
	TransactionID uint32
}

type udpConnectionResp struct {
	Action        uint32
	TransactionID uint32
	ConnectionID  uint64
}

type udpAnnounceReq struct {
	ConnectionID  uint64
	Action        uint32
	TransactionID uint32
	InfoHash      [20]byte
	PeerID        [20]byte
	Downloaded    uint64
	Left          uint64
	Uploaded      uint64
	Event         uint32
	IPAddress     uint32
	Key           uint32
	NumWant       int32
	Port          uint16
}

// requestPeersUDP handles the full UDP tracker communication process.
func requestPeersUDP(torrent TorrentFile, client Client) ([]byte, error) {
	parsedURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveUDPAddr("udp", parsedURL.Host)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Step 1: Connect
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	transactionID := random.Uint32()

	connReq := udpConnectionReq{
		ConnectionID:  magicConnectionID,
		Action:        connectAction,
		TransactionID: transactionID,
	}

	var connReqBuf bytes.Buffer
	if err := binary.Write(&connReqBuf, binary.BigEndian, connReq); err != nil {
		return nil, err
	}

	_, err = conn.WriteTo(connReqBuf.Bytes(), addr)
	if err != nil {
		return nil, err
	}

	connRespBuf := make([]byte, 16)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFrom(connRespBuf)
	if err != nil {
		return nil, err
	}

	if n < 16 {
		return nil, errors.New("udp tracker: connection response too short")
	}

	var connResp udpConnectionResp
	if err := binary.Read(bytes.NewReader(connRespBuf), binary.BigEndian, &connResp); err != nil {
		return nil, err
	}

	if connResp.Action != connectAction || connResp.TransactionID != transactionID {
		return nil, errors.New("udp tracker: invalid connection response")
	}

	// Step 2: Announce
	announceTransactionID := random.Uint32()
	var peerID [20]byte
	copy(peerID[:], []byte(client.PeerID))

	announceReq := udpAnnounceReq{
		ConnectionID:  connResp.ConnectionID,
		Action:        announceAction,
		TransactionID: announceTransactionID,
		InfoHash:      torrent.InfoHash,
		PeerID:        peerID,
		Downloaded:    0,
		Left:          uint64(torrent.Info.Length),
		Uploaded:      0,
		Event:         2, // started
		IPAddress:     0,
		Key:           0,
		NumWant:       -1,
		Port:          uint16(client.Port),
	}

	var announceReqBuf bytes.Buffer
	if err := binary.Write(&announceReqBuf, binary.BigEndian, announceReq); err != nil {
		return nil, err
	}

	_, err = conn.WriteTo(announceReqBuf.Bytes(), addr)
	if err != nil {
		return nil, err
	}

	announceRespBuf := make([]byte, 2048)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err = conn.ReadFrom(announceRespBuf)
	if err != nil {
		return nil, err
	}

	if n < 8 {
		return nil, errors.New("udp tracker: announce response too short")
	}

	// Check for error action from tracker
	var action uint32
	if err := binary.Read(bytes.NewReader(announceRespBuf[:4]), binary.BigEndian, &action); err != nil {
		return nil, err
	}
	if action == errorAction {
		return nil, fmt.Errorf("udp tracker error: %s", string(announceRespBuf[8:n]))
	}

	return announceRespBuf[:n], nil
}

