package config

import (
	"strings"
	"testing"
)

const validYAML = `
version: 1
profiles:
  - name: default
    listen: "127.0.0.1:40443"
    connect:
      host: my-proxy.example.com
      port: 443
      fallback_ips: ["188.114.98.0", "104.16.0.1"]
    spoof:
      strategy: wrong_seq
      sni_pool:
        - auth.vercel.com
        - cdn.segment.io
      randomize_timing: true
`

func TestParseValid(t *testing.T) {
	c, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c.Active != "default" {
		t.Fatalf("Active: got %q want default", c.Active)
	}
	p, ok := c.Lookup("default")
	if !ok {
		t.Fatal("Lookup failed")
	}
	if len(p.EffectiveSNIPool()) != 2 {
		t.Fatalf("SNI pool size: %d", len(p.EffectiveSNIPool()))
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		name, yaml, wantSubstr string
	}{
		{
			"missing_profiles",
			`version: 1`,
			"no profiles",
		},
		{
			"bad_listen",
			`version: 1
profiles:
  - name: x
    listen: "not-an-addrport"
    connect: {host: a.b, port: 443}
    spoof: {strategy: wrong_seq, sni: x.io}`,
			"invalid listen",
		},
		{
			"unknown_strategy",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a.b, port: 443}
    spoof: {strategy: nonsense, sni: x.io}`,
			"invalid strategy",
		},
		{
			"no_sni",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a.b, port: 443}
    spoof: {strategy: wrong_seq}`,
			"sni_pool or sni",
		},
		{
			"duplicate_name",
			`version: 1
profiles:
  - {name: x, listen: "127.0.0.1:1", connect: {host: a, port: 1}, spoof: {strategy: wrong_seq, sni: a}}
  - {name: x, listen: "127.0.0.1:2", connect: {host: a, port: 1}, spoof: {strategy: wrong_seq, sni: a}}`,
			"duplicate profile name",
		},
		{
			"unknown_active",
			`version: 1
active: ghost
profiles:
  - {name: x, listen: "127.0.0.1:1", connect: {host: a, port: 1}, spoof: {strategy: wrong_seq, sni: a}}`,
			"active profile",
		},
		{
			"oversized_pad",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a, port: 1}
    spoof: {strategy: wrong_seq, sni: a, max_extra_pad: 80000}`,
			"max_extra_pad",
		},
		{
			"min_gt_max_pad",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a, port: 1}
    spoof: {strategy: wrong_seq, sni: a, min_extra_pad: 500, max_extra_pad: 100}`,
			"max_extra_pad",
		},
		{
			"negative_min_delay",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a, port: 1}
    spoof: {strategy: wrong_seq, sni: a, min_delay: -1s}`,
			"non-negative",
		},
		{
			"inverted_delay",
			`version: 1
profiles:
  - name: x
    listen: "127.0.0.1:1"
    connect: {host: a, port: 1}
    spoof: {strategy: wrong_seq, sni: a, min_delay: 5ms, max_delay: 1ms}`,
			"max_delay",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.wantSubstr) {
				t.Fatalf("err %q does not contain %q", err, c.wantSubstr)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	y := `
version: 1
profiles:
  - name: only
    listen: "127.0.0.1:9"
    connect: {host: x.io}
    spoof: {strategy: wrong_seq, sni: a.io}`
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	p := &c.Profiles[0]
	if p.Connect.Port != 443 {
		t.Errorf("default port: got %d want 443", p.Connect.Port)
	}
	if p.Health.Interval == 0 {
		t.Error("default Health.Interval not set")
	}
	if c.Log.Level != "info" {
		t.Errorf("default log level: got %q", c.Log.Level)
	}
	if c.Active != "only" {
		t.Errorf("default active: got %q", c.Active)
	}
}
