package peers

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	bencode "github.com/jackpal/bencode-go"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

func urlEncodeInfoHash(hash [20]byte) string {
	var buf bytes.Buffer
	for _, b := range hash {
		// Escape every byte
		buf.WriteString(fmt.Sprintf("%%%02X", b))
	}
	return buf.String()
}

func RequestPeers(torrent TorrentFile, client Client) ([]byte, error) {
	parsedURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, err
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

