package sid

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

const (
	SidHeader = "X-Sid"
	SidKey    = "sid"
)

// Generator generates session IDs based on server IP, timestamp and atomic sequence
type Generator struct {
	secret []byte
	ip     string
	seq    uint64
}

// NewGenerator creates a new sid Generator
func NewGenerator(secret, ip string) *Generator {
	return &Generator{
		secret: []byte(secret),
		ip:     ip,
	}
}

// Generate creates a new sid from server IP, timestamp and sequence
// Format: base64(signature|ip|timestamp|seq) where signature = first 8 bytes of HMAC-SHA256
func (g *Generator) Generate(timestamp time.Time) string {
	seq := atomic.AddUint64(&g.seq, 1)

	// Build data: ip(4) + timestamp(8) + seq(8) = 20 bytes
	var data [20]byte
	ipBytes := net.ParseIP(g.ip).To4()
	if ipBytes != nil {
		copy(data[0:4], ipBytes[0:4])
	}
	binary.BigEndian.PutUint64(data[4:12], uint64(timestamp.Unix()))
	binary.BigEndian.PutUint64(data[12:20], seq)

	// Calculate HMAC-SHA256, take first 8 bytes as signature
	h := hmac.New(sha256.New, g.secret)
	h.Write(data[:])
	sig := h.Sum(nil) // 32 bytes

	// Build final: sig(8) + data(20) = 28 bytes
	var buf [28]byte
	copy(buf[0:8], sig[0:8])
	copy(buf[8:28], data[:])

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(buf[:])
}

// Parse extracts ip, timestamp and sequence from sid
func (g *Generator) Parse(sid string) (ip string, timestamp time.Time, seq uint64, err error) {
	data, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(sid)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("invalid sid encoding: %w", err)
	}
	if len(data) != 28 {
		return "", time.Time{}, 0, fmt.Errorf("invalid sid length: got %d, want 28", len(data))
	}

	// Extract and verify signature
	expectedSig := data[0:8]
	actualData := data[8:28]

	h := hmac.New(sha256.New, g.secret)
	h.Write(actualData)
	actualSig := h.Sum(nil)
	if !hmac.Equal(expectedSig, actualSig[0:8]) {
		return "", time.Time{}, 0, fmt.Errorf("signature mismatch")
	}

	// Decode IP (IPv4)
	ip = net.IP(actualData[0:4]).String()

	// Decode timestamp
	timestamp = time.Unix(int64(binary.BigEndian.Uint64(actualData[4:12])), 0)

	// Decode sequence
	seq = binary.BigEndian.Uint64(actualData[12:20])

	return ip, timestamp, seq, nil
}

// Verify checks if the sid signature is valid
func (g *Generator) Verify(sid string) bool {
	_, _, _, err := g.Parse(sid)
	return err == nil
}

// IsValid checks if the sid is valid (signature correct and not expired)
func (g *Generator) IsValid(sid string) bool {
	if !g.Verify(sid) {
		return false
	}
	_, ts, _, err := g.Parse(sid)
	if err != nil {
		return false
	}
	return time.Since(ts) < 7*24*time.Hour
}
