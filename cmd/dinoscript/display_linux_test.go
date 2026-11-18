package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLocalDisplay(t *testing.T) {
	for _, tc := range []struct{ display, socket, suffix string }{
		{":0", "/tmp/.X11-unix/X0", "0"},
		{"unix:3.1", "/tmp/.X11-unix/X3", "3.1"},
		{"unix/:2", "/tmp/.X11-unix/X2", "2"},
		{"workstation:0", "", ""},
		{"tcp/localhost:10", "", ""},
		{"/tmp/custom:8", "", ""},
		{":0.invalid", "", ""},
		{"", "", ""},
	} {
		socket, suffix, ok := localDisplay(tc.display)
		if socket != tc.socket || suffix != tc.suffix || ok != (tc.socket != "") {
			t.Errorf("localDisplay(%q) = (%q, %q, %t)", tc.display, socket, suffix, ok)
		}
	}
}

func TestAbstractDisplayBridge(t *testing.T) {
	upstream := "@dinoscript-test-" + strconv.Itoa(os.Getpid())
	server, err := net.ListenUnix("unix", &net.UnixAddr{Name: upstream, Net: "unix"})
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		t.Skipf("socket creation is blocked in this sandbox: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	server.SetDeadline(time.Now().Add(2 * time.Second))

	bridge, err := bridgeDisplay(upstream, "7.1")
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.close()
	display := bridge.address
	if !strings.HasSuffix(display, ":7.1") {
		t.Fatalf("bridge lost the display number or screen: %q", display)
	}
	path := strings.TrimSuffix(display, ".1")
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	peer, err := server.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(2 * time.Second))

	bridge.stopAccepting()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the socket to disappear when the bridge stops accepting connections: %v", err)
	}
	for _, pair := range [][2]net.Conn{{client, peer}, {peer, client}} {
		if _, err := fmt.Fprint(pair[0], "X11"); err != nil {
			t.Fatal(err)
		}
		raw := make([]byte, 3)
		if _, err := io.ReadFull(pair[1], raw); err != nil || string(raw) != "X11" {
			t.Fatalf("forwarded %q, error %v", raw, err)
		}
	}
}
