// SPDX-License-Identifier: MIT

package s7commctl

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/otfabric/s7commctl/internal/socks5test"
)

// Every command that connects goes through --proxy: the proxy sees a CONNECT
// to the PLC and the command succeeds.
func TestProxy_EveryConnectingCommand(t *testing.T) {
	p := probePLC(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"identify", p.args("identify", "--rack", "0", "--slot", "2")},
		{"probe rackslot", probeArgs(p)},
		{"compare read", compareArgs(p)},
		{"profile", []string{"profile", "--ip", "127.0.0.1", "--port", p.port, "--timeout", "2000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			px := socks5test.Start(t)
			runCLI(t, append(tc.args, "--proxy", px.URL())...).ok(t)
			reqs := px.Requests()
			if len(reqs) == 0 {
				t.Fatal("the proxy saw no CONNECT")
			}
			for _, r := range reqs {
				if r.Host != "127.0.0.1" || net.JoinHostPort(r.Host, strconv.Itoa(r.Port)) != net.JoinHostPort("127.0.0.1", p.port) {
					t.Errorf("CONNECT %+v, want 127.0.0.1:%s", r, p.port)
				}
			}
		})
	}
}

// discover scans a whole range through the proxy, which forwards every
// CONNECT to the fake PLC: client.Discover runs unmodified.
func TestProxy_DiscoverThroughProxy(t *testing.T) {
	p := probePLC(t)
	px := socks5test.Start(t)
	px.Forward = func(string) string { return net.JoinHostPort("127.0.0.1", p.port) }
	r := runCLI(t, "discover", "--cidr", "192.0.2.0/30", "--profile", "full", "--timeout", "2000",
		"--parallel", "1", "--quiet", "--proxy", px.URL()).ok(t)
	if !strings.Contains(r.stdout, "192.0.2.") {
		t.Fatalf("stdout = %q, want hosts of 192.0.2.0/30", r.stdout)
	}
	for _, req := range px.Requests() {
		if !strings.HasPrefix(req.Host, "192.0.2.") || req.Port != 102 {
			t.Errorf("CONNECT %+v, want 192.0.2.x:102", req)
		}
	}
}

// A proxy that refuses is a connection failure, not a usage error.
func TestProxy_RefusedIsAFailure(t *testing.T) {
	p := probePLC(t)
	px := socks5test.Start(t)
	px.Reply = 2
	runCLI(t, append(p.args("identify", "--rack", "0", "--slot", "2"), "--proxy", px.URL())...).
		wantFailure(t, "connection not allowed by ruleset")
}

// --source-addr binds the local end of the connection.
func TestSourceAddr(t *testing.T) {
	p := probePLC(t)
	runCLI(t, append(p.args("identify", "--rack", "0", "--slot", "2"), "--source-addr", "127.0.0.1")...).ok(t)
}

// An invalid --proxy or --source-addr is a usage error (exit 2) on every
// command that connects, before anything is sent.
func TestDialFlags_InvalidIsUsage(t *testing.T) {
	commands := [][]string{
		{"identify", "--ip", "127.0.0.1"},
		{"read", "--ip", "127.0.0.1", "--area", "DB", "--db", "1", "--size", "1"},
		{"probe", "rackslot", "--ip", "127.0.0.1"},
		{"compare", "read", "--ip", "127.0.0.1", "--candidates", "0/1", "--area", "DB", "--size", "1"},
		{"profile", "--ip", "127.0.0.1"},
		{"discover", "--cidr", "192.0.2.0/30"},
	}
	for _, bad := range [][]string{
		{"--proxy", "http://127.0.0.1:8080"},
		{"--proxy", "socks5://user:pw@127.0.0.1:1080"},
		{"--source-addr", "eth0"},
	} {
		for _, cmd := range commands {
			t.Run(strings.Join(append(cmd[:1:1], bad...), " "), func(t *testing.T) {
				runCLI(t, append(append([]string{}, cmd...), bad...)...).wantUsage(t, bad[0])
			})
		}
	}
}
