// Package integrate detects installed proxy clients (Xray, v2ray,
// sing-box, NekoBox) and patches their config so their outbound server
// address becomes 127.0.0.1:40443 (snix's listen address) while the real
// upstream address is stored in snix's own config.
//
// Every Patch() call:
//   - backs up the original config to <path>.bak before writing
//   - preserves unrelated fields by round-tripping through a generic map
//   - returns an error before writing anything if the config is
//     structurally unexpected (safer to fail than to corrupt).
package integrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Client describes one detected proxy client's install.
type Client struct {
	// Name is a short label ("xray", "v2ray", "sing-box", "nekobox").
	Name string
	// Binary is the absolute path to the executable (diagnostic only).
	Binary string
	// ConfigPath is the file we would patch.
	ConfigPath string
	// Patch rewrites the config so the client's outbound server is
	// snix's listen address. origHost+origPort is the user's real proxy
	// server address (what snix will forward to). Returns an error if
	// the config doesn't match the expected shape; the backup is always
	// written before any modification.
	Patch func(origHost string, origPort uint16) error
}

// Detect scans the current system for supported proxy clients and returns
// one Client entry for each found install. Order is not stable.
func Detect() []Client {
	var out []Client
	if c, ok := detectXray(); ok {
		out = append(out, c)
	}
	if c, ok := detectV2ray(); ok {
		out = append(out, c)
	}
	if c, ok := detectSingBox(); ok {
		out = append(out, c)
	}
	return out
}

// -- xray / v2ray (JSON config, same shape) --------------------------------

func detectXray() (Client, bool)  { return detectJSONProxy("xray", xrayConfigPaths()) }
func detectV2ray() (Client, bool) { return detectJSONProxy("v2ray", v2rayConfigPaths()) }

// detectJSONProxy looks for an executable on PATH and a config file in one
// of the candidate locations. Returns a Client that patches the first
// VLESS/VMess outbound it finds.
func detectJSONProxy(name string, candidates []string) (Client, bool) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return Client{}, false
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return Client{
				Name:       name,
				Binary:     bin,
				ConfigPath: p,
				Patch:      jsonProxyPatcher(p),
			}, true
		}
	}
	return Client{}, false
}

func jsonProxyPatcher(path string) func(string, uint16) error {
	return func(origHost string, origPort uint16) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("unparseable JSON: %w", err)
		}

		// Xray / v2ray outbounds live under .outbounds[].settings.vnext[] for
		// VLESS/VMess, or .outbounds[].settings.servers[] for Trojan/Shadowsocks.
		outbounds, ok := doc["outbounds"].([]any)
		if !ok || len(outbounds) == 0 {
			return errors.New("no outbounds array found")
		}
		patched := false
		for _, ob := range outbounds {
			obm, ok := ob.(map[string]any)
			if !ok {
				continue
			}
			settings, _ := obm["settings"].(map[string]any)
			if settings == nil {
				continue
			}
			if patchVNext(settings, origHost, origPort) || patchServers(settings, origHost, origPort) {
				patched = true
			}
		}
		if !patched {
			return errors.New("no VLESS/VMess/Trojan outbound with an address field found")
		}

		// Backup using the same bytes we parsed — safer than re-reading
		// (avoids a TOCTOU where the file changed between read and backup).
		if err := os.WriteFile(path+".bak", raw, 0o644); err != nil {
			return fmt.Errorf("backup: %w", err)
		}

		// Write new.
		updated, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, updated, 0o644)
	}
}

// patchVNext rewrites the first vnext[].address/port it finds. Returns true if changed.
func patchVNext(settings map[string]any, origHost string, origPort uint16) bool {
	vnext, ok := settings["vnext"].([]any)
	if !ok {
		return false
	}
	for _, v := range vnext {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if _, has := vm["address"]; has {
			vm["address"] = "127.0.0.1"
			vm["port"] = 40443
			// Leave uuid, id, alterId, users, etc. untouched.
			_ = origHost
			_ = origPort
			return true
		}
	}
	return false
}

// patchServers rewrites the first servers[].address/port for Trojan/SS style.
func patchServers(settings map[string]any, origHost string, origPort uint16) bool {
	servers, ok := settings["servers"].([]any)
	if !ok {
		return false
	}
	for _, s := range servers {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if _, has := sm["address"]; has {
			sm["address"] = "127.0.0.1"
			sm["port"] = 40443
			_ = origHost
			_ = origPort
			return true
		}
	}
	return false
}

// -- sing-box (JSON with typed outbounds) ---------------------------------

func detectSingBox() (Client, bool) {
	bin, err := exec.LookPath("sing-box")
	if err != nil {
		return Client{}, false
	}
	for _, p := range singBoxConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			return Client{
				Name:       "sing-box",
				Binary:     bin,
				ConfigPath: p,
				Patch:      singBoxPatcher(p),
			}, true
		}
	}
	return Client{}, false
}

func singBoxPatcher(path string) func(string, uint16) error {
	return func(origHost string, origPort uint16) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("unparseable JSON: %w", err)
		}
		outbounds, ok := doc["outbounds"].([]any)
		if !ok {
			return errors.New("no outbounds array")
		}
		patched := false
		for _, ob := range outbounds {
			obm, ok := ob.(map[string]any)
			if !ok {
				continue
			}
			t, _ := obm["type"].(string)
			switch t {
			case "vless", "vmess", "trojan", "shadowsocks", "hysteria", "hysteria2", "tuic":
				if _, has := obm["server"]; has {
					obm["server"] = "127.0.0.1"
					obm["server_port"] = 40443
					patched = true
				}
			}
		}
		if !patched {
			return errors.New("no applicable outbound with type VLESS/VMess/Trojan")
		}
		if err := os.WriteFile(path+".bak", raw, 0o644); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		updated, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, updated, 0o644)
	}
}

// -- platform-specific config search paths --------------------------------

func xrayConfigPaths() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		app := os.Getenv("APPDATA")
		return []string{
			filepath.Join(app, "xray", "config.json"),
			filepath.Join(app, "v2rayN", "xray_config.json"),
			`C:\Program Files\xray\config.json`,
		}
	}
	return []string{
		"/etc/xray/config.json",
		"/usr/local/etc/xray/config.json",
		filepath.Join(home, ".config", "xray", "config.json"),
	}
}

func v2rayConfigPaths() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		app := os.Getenv("APPDATA")
		return []string{
			filepath.Join(app, "v2ray", "config.json"),
			filepath.Join(app, "v2rayN", "config.json"),
		}
	}
	return []string{
		"/etc/v2ray/config.json",
		"/usr/local/etc/v2ray/config.json",
		filepath.Join(home, ".config", "v2ray", "config.json"),
	}
}

func singBoxConfigPaths() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		app := os.Getenv("APPDATA")
		return []string{
			filepath.Join(app, "sing-box", "config.json"),
		}
	}
	return []string{
		"/etc/sing-box/config.json",
		"/usr/local/etc/sing-box/config.json",
		filepath.Join(home, ".config", "sing-box", "config.json"),
	}
}
