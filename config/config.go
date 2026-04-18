// Package config loads, validates, and (in future phases) hot-reloads snix
// configuration. A config is a list of named Profiles; exactly one is
// "active" at a time but switching is instantaneous and does not drop
// existing flows.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/SamNet-dev/snix/core/bypass"
)

// Config is the top-level YAML document.
type Config struct {
	// Version pins the schema so we can evolve without silent breakage.
	Version int `yaml:"version"`
	// Active names the Profile to run on startup. If empty, the first
	// profile is used.
	Active string `yaml:"active,omitempty"`
	// Profiles is the set of named server bundles.
	Profiles []Profile `yaml:"profiles"`
	// Log controls output verbosity.
	Log LogConfig `yaml:"log,omitempty"`
}

// LogConfig configures the logger.
type LogConfig struct {
	// Level is "debug", "info", "warn", or "error".
	Level string `yaml:"level,omitempty"`
	// File, if set, writes logs there in addition to stderr.
	File string `yaml:"file,omitempty"`
}

// Profile describes one bypass target plus the spoofing strategy to use.
type Profile struct {
	// Name uniquely identifies this profile.
	Name string `yaml:"name"`
	// Listen is the local address the proxy binds to (e.g. "127.0.0.1:40443").
	Listen string `yaml:"listen"`
	// Connect is the upstream server the relay talks to.
	Connect ConnectConfig `yaml:"connect"`
	// Spoof configures the DPI bypass applied during the handshake.
	Spoof SpoofConfig `yaml:"spoof"`
	// Health configures passive failover behaviour.
	Health HealthConfig `yaml:"health,omitempty"`
}

// ConnectConfig describes the upstream server.
type ConnectConfig struct {
	// Host is a DNS name or IP literal. If Host resolves to multiple A/AAAA
	// records, FallbackIPs is merged in.
	Host string `yaml:"host"`
	// Port on the upstream (typically 443).
	Port uint16 `yaml:"port"`
	// FallbackIPs are additional IPs to try if Host fails or is blocked.
	FallbackIPs []string `yaml:"fallback_ips,omitempty"`
}

// SpoofConfig configures the DPI bypass strategy.
type SpoofConfig struct {
	// Strategy selects the bypass algorithm; see core/bypass for names.
	// If StrategyRotation is non-empty this is the fallback used only when
	// rotation is disabled.
	Strategy bypass.Name `yaml:"strategy"`
	// StrategyRotation, if non-empty, randomizes the strategy per flow.
	// Typical: ["wrong_seq", "wrong_checksum"].
	StrategyRotation []bypass.Name `yaml:"strategy_rotation,omitempty"`
	// SNIPool is the list of fake SNI values to rotate through. If empty,
	// SNI is required instead.
	SNIPool []string `yaml:"sni_pool,omitempty"`
	// SNI is a single fake SNI; only used if SNIPool is empty.
	SNI string `yaml:"sni,omitempty"`
	// SNISelection is "random" (default) or "round_robin".
	SNISelection string `yaml:"sni_selection,omitempty"`

	// RandomizeTiming jitters the inject delay; MinDelay/MaxDelay bound it.
	RandomizeTiming bool          `yaml:"randomize_timing,omitempty"`
	MinDelay        time.Duration `yaml:"min_delay,omitempty"`
	MaxDelay        time.Duration `yaml:"max_delay,omitempty"`

	// RandomizePadding varies the ClientHello size; the fake packet gets
	// [MinExtraPad, MaxExtraPad] bytes of additional padding on top of 517.
	RandomizePadding bool `yaml:"randomize_padding,omitempty"`
	MinExtraPad      int  `yaml:"min_extra_pad,omitempty"`
	MaxExtraPad      int  `yaml:"max_extra_pad,omitempty"`

	// IPIDDeltaRange is the upper bound on the IP ID delta applied to the
	// injected packet. 1 (default) matches upstream; higher breaks the
	// predictable +1 pattern.
	IPIDDeltaRange int `yaml:"ip_id_delta_range,omitempty"`
}

// HealthConfig tunes the background health monitor.
type HealthConfig struct {
	// Interval between probes. Default 30s.
	Interval time.Duration `yaml:"interval,omitempty"`
	// AutoFailover rotates to the next FallbackIP on sustained failure.
	AutoFailover bool `yaml:"auto_failover,omitempty"`
}

// Load reads and validates a YAML config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	return Parse(b)
}

// Parse validates a YAML document in memory. Exported so the TUI can
// validate as the user types.
func Parse(doc []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(doc, &c); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks invariants and sets defaults in-place.
func (c *Config) Validate() error {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Version != 1 {
		return fmt.Errorf("config: unsupported version %d (want 1)", c.Version)
	}
	if len(c.Profiles) == 0 {
		return errors.New("config: no profiles defined")
	}
	names := make(map[string]struct{}, len(c.Profiles))
	for i := range c.Profiles {
		p := &c.Profiles[i]
		if p.Name == "" {
			return fmt.Errorf("config: profile[%d]: name is required", i)
		}
		if _, dup := names[p.Name]; dup {
			return fmt.Errorf("config: duplicate profile name %q", p.Name)
		}
		names[p.Name] = struct{}{}
		if err := p.validate(); err != nil {
			return fmt.Errorf("config: profile %q: %w", p.Name, err)
		}
	}
	if c.Active == "" {
		c.Active = c.Profiles[0].Name
	}
	if _, ok := names[c.Active]; !ok {
		return fmt.Errorf("config: active profile %q not found", c.Active)
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	return nil
}

func (p *Profile) validate() error {
	if p.Listen == "" {
		return errors.New("listen is required")
	}
	if _, err := netip.ParseAddrPort(p.Listen); err != nil {
		return fmt.Errorf("invalid listen %q: %w", p.Listen, err)
	}
	if p.Connect.Host == "" {
		return errors.New("connect.host is required")
	}
	if p.Connect.Port == 0 {
		p.Connect.Port = 443
	}
	for _, ip := range p.Connect.FallbackIPs {
		if _, err := netip.ParseAddr(ip); err != nil {
			return fmt.Errorf("invalid fallback_ip %q: %w", ip, err)
		}
	}
	if _, err := bypass.ByName(p.Spoof.Strategy); err != nil {
		return fmt.Errorf("invalid strategy: %w", err)
	}
	for _, s := range p.Spoof.StrategyRotation {
		if _, err := bypass.ByName(s); err != nil {
			return fmt.Errorf("invalid strategy_rotation entry %q: %w", s, err)
		}
	}
	switch p.Spoof.SNISelection {
	case "", "random", "round_robin":
		// ok
	default:
		return fmt.Errorf("invalid sni_selection %q (want random or round_robin)", p.Spoof.SNISelection)
	}
	if len(p.Spoof.SNIPool) == 0 && p.Spoof.SNI == "" {
		return errors.New("spoof: either sni_pool or sni must be set")
	}
	for _, s := range p.Spoof.SNIPool {
		if len(s) == 0 || len(s) > 219 {
			return fmt.Errorf("sni %q: length must be 1..219", s)
		}
	}
	if p.Spoof.SNI != "" && (len(p.Spoof.SNI) > 219) {
		return fmt.Errorf("sni too long")
	}
	// Padding bounds — the injector clips at 65000 bytes of extra padding.
	// Anything larger would also fragment at the IP layer, defeating the
	// "looks like one handshake packet" invariant. Clamp user input.
	if p.Spoof.MinExtraPad < 0 {
		return fmt.Errorf("min_extra_pad must be >= 0")
	}
	if p.Spoof.MaxExtraPad < 0 || p.Spoof.MaxExtraPad > 65000 {
		return fmt.Errorf("max_extra_pad must be in [0, 65000]")
	}
	if p.Spoof.MaxExtraPad < p.Spoof.MinExtraPad {
		return fmt.Errorf("max_extra_pad (%d) < min_extra_pad (%d)",
			p.Spoof.MaxExtraPad, p.Spoof.MinExtraPad)
	}
	// Timing bounds — reject negatives and MaxDelay < MinDelay.
	if p.Spoof.MinDelay < 0 || p.Spoof.MaxDelay < 0 {
		return fmt.Errorf("min_delay and max_delay must be non-negative")
	}
	if p.Spoof.MaxDelay > 0 && p.Spoof.MaxDelay < p.Spoof.MinDelay {
		return fmt.Errorf("max_delay (%s) < min_delay (%s)",
			p.Spoof.MaxDelay, p.Spoof.MinDelay)
	}
	if p.Health.Interval == 0 {
		p.Health.Interval = 30 * time.Second
	}
	return nil
}

// Lookup returns the profile with the given name.
func (c *Config) Lookup(name string) (*Profile, bool) {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i], true
		}
	}
	return nil, false
}

// EffectiveSNIPool returns the spoof SNI candidates with SNI promoted to a
// single-entry pool when SNIPool is empty.
func (p *Profile) EffectiveSNIPool() []string {
	if len(p.Spoof.SNIPool) > 0 {
		return p.Spoof.SNIPool
	}
	if p.Spoof.SNI != "" {
		return []string{p.Spoof.SNI}
	}
	return nil
}
