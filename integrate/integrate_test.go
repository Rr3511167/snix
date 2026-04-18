package integrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPatchVNextRewritesAddressAndPort verifies the Xray/v2ray VLESS shape
// is patched in-place and unrelated fields are preserved.
func TestPatchVNextRewritesAddressAndPort(t *testing.T) {
	src := []byte(`{
  "log": {"loglevel": "info"},
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "myproxy.example.com",
            "port": 443,
            "users": [{"id": "abc", "encryption": "none"}]
          }
        ]
      },
      "streamSettings": {"network": "tcp"}
    }
  ]
}`)
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, src, 0o644); err != nil {
		t.Fatal(err)
	}

	patch := jsonProxyPatcher(p)
	if err := patch("myproxy.example.com", 443); err != nil {
		t.Fatalf("patch: %v", err)
	}

	// Backup exists.
	if _, err := os.Stat(p + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	// Re-read and verify.
	got, _ := os.ReadFile(p)
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	ob := doc["outbounds"].([]any)[0].(map[string]any)
	settings := ob["settings"].(map[string]any)
	vn := settings["vnext"].([]any)[0].(map[string]any)
	if vn["address"] != "127.0.0.1" {
		t.Fatalf("address: got %v want 127.0.0.1", vn["address"])
	}
	if vn["port"].(float64) != 40443 {
		t.Fatalf("port: got %v want 40443", vn["port"])
	}
	// Untouched fields survived.
	users := vn["users"].([]any)[0].(map[string]any)
	if users["id"] != "abc" {
		t.Fatalf("users.id clobbered: %v", users["id"])
	}
	if ob["streamSettings"] == nil {
		t.Fatal("streamSettings dropped")
	}
	if doc["log"] == nil {
		t.Fatal("log section dropped")
	}
}

func TestPatchRejectsConfigWithoutOutbounds(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(`{"log":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsonProxyPatcher(p)("x", 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestSingBoxPatcher(t *testing.T) {
	src := []byte(`{
  "outbounds": [
    {"type": "direct"},
    {"type": "vless", "server": "origin.example.com", "server_port": 443, "uuid": "u"}
  ]
}`)
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := singBoxPatcher(p)("origin.example.com", 443); err != nil {
		t.Fatalf("patch: %v", err)
	}
	raw, _ := os.ReadFile(p)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	vless := doc["outbounds"].([]any)[1].(map[string]any)
	if vless["server"] != "127.0.0.1" {
		t.Fatalf("server: %v", vless["server"])
	}
	if vless["server_port"].(float64) != 40443 {
		t.Fatalf("server_port: %v", vless["server_port"])
	}
	if vless["uuid"] != "u" {
		t.Fatalf("uuid clobbered: %v", vless["uuid"])
	}
}
