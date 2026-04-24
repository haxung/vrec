package sid

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")
	timestamp := time.Now()

	sid := g.Generate(timestamp)

	if sid == "" {
		t.Error("sid should not be empty")
	}

	// 验证格式: base64(sig|ip|timestamp|seq) = 28 bytes
	data, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(sid)
	if err != nil {
		t.Fatalf("sid should be valid base64: %v", err)
	}
	if len(data) != 28 {
		t.Fatalf("expected 28 bytes, got %d", len(data))
	}
}

func TestGenerator_Generate_Uniqueness(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")

	var sids []string
	for range 100 {
		sids = append(sids, g.Generate(time.Now()))
	}

	seen := make(map[string]bool)
	for _, sid := range sids {
		if seen[sid] {
			t.Errorf("duplicate sid generated: %s", sid)
		}
		seen[sid] = true
	}
}

func TestGenerator_Generate_Concurrent(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")

	var wg sync.WaitGroup
	const numGoroutines = 100
	const sidsPerGoroutine = 100

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range sidsPerGoroutine {
				_ = g.Generate(time.Now())
			}
		}()
	}

	wg.Wait()
}

func TestGenerator_Parse(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")
	timestamp := time.Now()

	sid := g.Generate(timestamp)
	ip, ts, seq, err := g.Parse(sid)

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if ip == "" {
		t.Error("ip should not be empty")
	}

	if ts.Unix() != timestamp.Unix() {
		t.Errorf("timestamp mismatch: expected %d, got %d", timestamp.Unix(), ts.Unix())
	}

	if seq == 0 {
		t.Error("sequence should be >= 1")
	}
}

func TestGenerator_Parse_InvalidFormat(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")

	testCases := []struct {
		name string
		sid  string
	}{
		{"empty", ""},
		{"invalid base64", "not-valid-base64!!!"},
		{"too short", strings.Repeat("a", 20)},
		{"too long", strings.Repeat("a", 50)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := g.Parse(tc.sid)
			if err == nil {
				t.Errorf("Parse(%q) should fail", tc.sid)
			}
		})
	}
}

func TestGenerator_Parse_Tampered(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")
	timestamp := time.Now()

	sid := g.Generate(timestamp)
	// Tamper with the sid
	data, _ := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(sid)
	data[20] ^= 0xFF // flip bits in the "data" portion
	tamperedSid := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)

	_, _, _, err := g.Parse(tamperedSid)
	if err == nil {
		t.Error("tampered sid should fail Parse (signature mismatch)")
	}
}

func TestGenerator_Verify(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")

	validSid := g.Generate(time.Now())
	if !g.Verify(validSid) {
		t.Error("valid sid should pass Verify")
	}

	if g.Verify("invalid") {
		t.Error("invalid sid should fail Verify")
	}
}

func TestGenerator_IsValid(t *testing.T) {
	g := NewGenerator("test-secret", "192.168.1.1")

	validSid := g.Generate(time.Now())
	if !g.IsValid(validSid) {
		t.Error("valid sid should pass IsValid")
	}

	// Manually create an expired sid (with old timestamp)
	g2 := NewGenerator("test-secret", "192.168.1.1")
	oldSid := g2.Generate(time.Now().Add(-8 * 24 * time.Hour))
	if g2.IsValid(oldSid) {
		t.Error("expired sid should fail IsValid")
	}

	if g.IsValid("invalid") {
		t.Error("invalid sid should fail IsValid")
	}
}

func TestGenerator_DifferentSecret(t *testing.T) {
	g1 := NewGenerator("secret1", "192.168.1.1")
	g2 := NewGenerator("secret2", "192.168.1.1")

	timestamp := time.Now()
	sid1 := g1.Generate(timestamp)
	sid2 := g2.Generate(timestamp)

	if sid1 == sid2 {
		t.Error("different secrets should produce different sids")
	}

	// Verify with wrong secret should fail
	_, _, _, err := g2.Parse(sid1)
	if err == nil {
		t.Error("sid generated with secret1 should not parse with secret2")
	}
}

func TestGenerator_DifferentIP(t *testing.T) {
	g1 := NewGenerator("test-secret", "192.168.1.1")
	g2 := NewGenerator("test-secret", "192.168.1.2")

	timestamp := time.Now()
	sid1 := g1.Generate(timestamp)
	sid2 := g2.Generate(timestamp)

	if sid1 == sid2 {
		t.Error("different IPs should produce different sids")
	}

	ip1, _, _, _ := g1.Parse(sid1)
	ip2, _, _, _ := g2.Parse(sid2)

	if ip1 == ip2 {
		t.Error("parsed IPs should be different")
	}
}

func TestConstants(t *testing.T) {
	if SidHeader != "X-Sid" {
		t.Errorf("SidHeader should be 'X-Sid', got %q", SidHeader)
	}
	if SidKey != "sid" {
		t.Errorf("SidKey should be 'sid', got %q", SidKey)
	}
}
