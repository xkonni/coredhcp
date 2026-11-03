// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package config

import (
	"net"
	"strings"
	"testing"
)

func loadConfig(yaml string) (*Config, error) {
	c := New()
	c.v.SetConfigType("yml")
	if err := c.v.ReadConfig(strings.NewReader(yaml)); err != nil {
		return nil, err
	}
	if err := c.parseConfig(protocolV4); err != nil {
		return nil, err
	}
	return c, nil
}

func TestSourceAddress(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantIP    net.IP
		wantError bool
	}{
		{
			name: "valid IPv4",
			yaml: `
server4:
  listen:
    - "0.0.0.0:67"
  source_address: "10.0.0.1"
  plugins:
    - lease_time: 3600s`,
			wantIP: net.IP{10, 0, 0, 1},
		},
		{
			name: "absent source_address",
			yaml: `
server4:
  listen:
    - "0.0.0.0:67"
  plugins:
    - lease_time: 3600s`,
			wantIP: nil,
		},
		{
			name: "invalid IP",
			yaml: `
server4:
  listen:
    - "0.0.0.0:67"
  source_address: "not-an-ip"
  plugins:
    - lease_time: 3600s`,
			wantError: true,
		},
		{
			name: "IPv6 rejected",
			yaml: `
server4:
  listen:
    - "0.0.0.0:67"
  source_address: "2001:db8::1"
  plugins:
    - lease_time: 3600s`,
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := loadConfig(tc.yaml)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantIP == nil {
				if c.Server4.SourceIP != nil {
					t.Errorf("expected nil SourceIP, got %v", c.Server4.SourceIP)
				}
				return
			}
			if !c.Server4.SourceIP.Equal(tc.wantIP) {
				t.Errorf("SourceIP = %v, want %v", c.Server4.SourceIP, tc.wantIP)
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	testcases := []struct {
		hostport string
		ip       string
		zone     string
		port     string
		err      bool // Should return an error (ie true for err != nil)
	}{
		{"0.0.0.0:67", "0.0.0.0", "", "67", false},
		{"192.0.2.0", "192.0.2.0", "", "", false},
		{"192.0.2.9%eth0", "192.0.2.9", "eth0", "", false},
		{"0.0.0.0%eth0:67", "0.0.0.0", "eth0", "67", false},
		{"0.0.0.0:20%eth0:67", "0.0.0.0", "eth0", "67", true},
		{"2001:db8::1:547", "", "", "547", true}, // [] mandatory for v6
		{"[::]:547", "::", "", "547", false},
		{"[fe80::1%eth0]", "fe80::1", "eth0", "", false},
		{"[fe80::1]:eth1", "fe80::1", "", "eth1", false},             // no validation of ports in this function
		{"fe80::1%eth0:547", "fe80::1", "eth0", "547", true},         // [] mandatory for v6 even with %zone
		{"fe80::1%eth0", "fe80::1", "eth0", "547", true},             // [] mandatory for v6 even without port
		{"[2001:db8::2]47", "fe80::1", "eth0", "547", true},          // garbage after []
		{"[ff02::1:2]%srv_u:547", "ff02::1:2", "srv_u", "547", true}, // FIXME: Linux `ss` format, we should accept but net.SplitHostPort doesn't
		{":http", "", "", "http", false},
		{"%eth0:80", "", "eth0", "80", false},          // janky, but looks valid enough for "[::%eth0]:80" imo
		{"%eth0", "", "eth0", "", false},               // janky
		{"fe80::1]:80", "fe80::1", "", "80", true},     // unbalanced ]
		{"fe80::1%eth0]", "fe80::1", "eth0", "", true}, // unbalanced ], no port
		{"", "", "", "", false},                        // trivial case, still valid
	}

	for _, tc := range testcases {
		ip, zone, port, err := splitHostPort(tc.hostport)
		if tc.err != (err != nil) {
			errState := "not "
			if tc.err {
				errState = ""
			}
			t.Errorf("Mismatched error state: %s should %serror (got err: %v)\n", tc.hostport, errState, err)
			continue
		}
		if err == nil && (ip != tc.ip || zone != tc.zone || port != tc.port) {
			t.Errorf("%s => \"%s\", \"%s\", \"%s\" expected \"%s\",\"%s\",\"%s\"\n", tc.hostport, ip, zone, port, tc.ip, tc.zone, tc.port)
		}
	}
}
