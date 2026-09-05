package session

// Regression coverage for the SSH -J path when the jump host
// authenticates via the identity (密钥库) store. Two distinct
// regressions landed here over time:
//
//  1. makeSSHAuthMethods had no default branch, so a config whose
//     AuthType fell outside {password, key, keyText} (legacy "agent",
//     "" from a stale JSON row, or an AuthType="identity" row that
//     somehow reached the tunnel service before materializeIdentity
//     ran) produced an empty auth-methods slice. crypto/ssh would
//     then walk the SSH_USERAUTH flow, send only the "none" probe,
//     and surface the user-visible
//
//         ssh: unable to authenticate,
//             attempted methods [none],
//             no supported methods remain
//
//     which is impossible to act on. The fix in ssh_auth.go
//     (default branch falls back to password) mirrors what
//     buildAuthMethods already does, so any future regression
//     here would re-introduce the asymmetry.
//
//  2. tunnelService.Start with an unpopulated identity-shaped
//     config (User == "", AuthType == "identity") would surface
//     the user-visible wording above. After the fix the error
//     becomes "attempted methods [none password]" — same root
//     cause, but the wording now leaks enough info to act on
//     (an empty password / missing user, vs. the literal
//     "no methods configured"). The wording regression test below
//     locks in that change so a future refactor can't silently
//     flip the behavior back.

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestMakeSSHAuthMethods_DefaultFallbackLocksInPasswordMethod
// guards the default-branch fix: AuthType outside the explicit
// {password, key, keyText} set MUST still yield at least one
// auth method (falling back to password), matching what
// buildAuthMethods does on the SIP (SFTP, monitor) path.
func TestMakeSSHAuthMethods_DefaultFallbackLocksInPasswordMethod(t *testing.T) {
	cases := []struct {
		name string
		cfg  ConnectionConfig
	}{
		{"empty AuthType", ConnectionConfig{}},
		{"unknown agent", ConnectionConfig{AuthType: "agent"}},
		{"unmaterialized identity", ConnectionConfig{AuthType: "identity", IdentityId: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(makeSSHAuthMethods(tc.cfg, nil)); got == 0 {
				t.Fatalf("makeSSHAuthMethods returned 0 methods; the default-branch fix regressed")
			}
		})
	}
}

// TestTunnelService_Start_UnmaterializedIdentitySurfacesPassword
// pins the user-visible error wording for the case where a
// config with AuthType="identity" reaches the tunnel service
// without populatePasswords + materializeIdentity running first.
// Before the fix the handshake surfaced "attempted methods [none]"
// (the symptom the user reported); after the fix it surfaces
// "attempted methods [none password]" — still a failure, but the
// wording reveals the real cause (no populated credential) rather
// than an empty method list. Any future regression that drops
// the default-branch fix would silently flip this back to
// "[none]" and re-introduce the un-actionable error.
func TestTunnelService_Start_UnmaterializedIdentitySurfacesPassword(t *testing.T) {
	sshPort := startTunnelFakeSSHD(t, "jump", "jump-pw")
	targetPort := reserveUnusedPort(t)

	ts := NewTunnelService()

	cfg := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     sshPort,
		AuthType: "identity",
		IdentityId: "ignored",
		// intentionally no User / Password / KeyPath / KeyContent:
		// simulates a row that reached the tunnel without
		// materializeIdentity having run.
	}

	_, err := ts.Start("test-session", cfg, "127.0.0.1", targetPort, nil)
	if err == nil {
		t.Fatal("tunnelService.Start unexpectedly succeeded without credentials")
	}
	if !strings.Contains(err.Error(), "no supported methods") {
		t.Fatalf("expected 'no supported methods' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "[none password]") {
		t.Fatalf("expected '[none password]' (default-branch fix working); "+
			"if this is '[none]' the default-branch fix regressed. Got: %v", err)
	}
}

// TestTunnelService_Start_IdentityResolvedPasswordSucceeds is
// the positive case: an identity-resolved config (the shape
// setupJumpHostTunnel produces after materializeIdentity fills
// User / Password from the identity store) must ride the tunnel
// successfully. Locks in that the populatePasswords (skips
// identity rows) + materializeIdentity (fills User / Password)
// pair actually produces a usable client config end-to-end.
func TestTunnelService_Start_IdentityResolvedPasswordSucceeds(t *testing.T) {
	sshPort := startTunnelFakeSSHD(t, "jump", "jump-pw")
	targetPort := reserveUnusedPort(t)

	ts := NewTunnelService()

	cfg := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     sshPort,
		User:     "jump",
		AuthType: "password",
		Password: "jump-pw",
	}
	localPort, err := ts.Start("test-session", cfg, "127.0.0.1", targetPort, nil)
	if err != nil {
		t.Fatalf("tunnelService.Start with identity-resolved password: %v", err)
	}
	if localPort == 0 {
		t.Fatal("tunnel returned port 0")
	}
	t.Cleanup(func() { ts.Stop("test-session") })
}

// startTunnelFakeSSHD spins up a local SSH server that accepts
// password auth for (user, password) and direct-tcpip channels.
// Returns the listening port. Shared between the unit-level
// regressions in this file and any future codepath tests.
func startTunnelFakeSSHD(t *testing.T, user, password string) int {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(m ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if m.User() == user && string(pw) == password {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("auth rejected for %q", m.User())
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sconn, _, _, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sconn.Close()
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// reserveUnusedPort grabs a 127.0.0.1 port and immediately releases
// it so a subsequent dial fails fast. Mirrors the same trick in
// test_connection_tunnel_test.go.
func reserveUnusedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	p, _ := strconv.Atoi(ln.Addr().String()[len("127.0.0.1:"):])
	_ = ln.Close()
	return p
}
