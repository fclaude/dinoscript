package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/richardwilkes/unison"
)

func prepareDisplay() (func(), error) {
	display := os.Getenv("DISPLAY")
	socket, suffix, local := localDisplay(display)
	if !local {
		return func() {}, nil
	}
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		return func() {}, nil
	}

	abstract := "@" + socket
	probe, err := net.DialTimeout("unix", abstract, time.Second)
	if err != nil {
		return nil, fmt.Errorf("can't connect to X11 display %s. Check that Xorg or XWayland is running: %w", display, err)
	}
	probe.Close()

	bridge, err := bridgeDisplay(abstract, suffix)
	if err != nil {
		return nil, err
	}
	if err := os.Setenv("DISPLAY", bridge.address); err != nil {
		bridge.close()
		return nil, err
	}
	// The relay carries the X11 byte stream. Software rendering avoids
	// graphics extensions that pass file descriptors over the original socket.
	if err := os.Setenv(unison.CPURenderingEnvKey, "1"); err != nil {
		bridge.close()
		return nil, err
	}
	// Unison calls its quit hook before shutting down its X11 connection.
	// Stop accepting new clients there and leave open streams to process exit.
	return bridge.stopAccepting, nil
}

func localDisplay(display string) (socket, suffix string, ok bool) {
	host, suffix, local := strings.Cut(display, ":")
	if !local || (host != "" && host != "unix" && host != "unix/") {
		return "", "", false
	}
	number, screen, _ := strings.Cut(suffix, ".")
	n, err := strconv.ParseUint(number, 10, 31)
	if err != nil {
		return "", "", false
	}
	if screen != "" {
		if _, err := strconv.ParseUint(screen, 10, 31); err != nil {
			return "", "", false
		}
	}
	return "/tmp/.X11-unix/X" + strconv.FormatUint(n, 10), suffix, true
}

type displayBridge struct {
	address       string
	stopAccepting func()
	close         func()
}

// bridgeDisplay gives Unison a private address for the same X11 display and cookie.
func bridgeDisplay(upstream, suffix string) (displayBridge, error) {
	dir, err := os.MkdirTemp("/tmp", "dinoscript-x11-")
	if err != nil {
		return displayBridge{}, err
	}
	base := filepath.Join(dir, "display") + ":"
	forwarded := base + suffix
	number, _, _ := strings.Cut(suffix, ".")
	listenPath := base + number
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: listenPath, Net: "unix"})
	if err != nil {
		os.Remove(dir)
		return displayBridge{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	var serving sync.WaitGroup
	serving.Go(func() { serveDisplay(ctx, listener, upstream) })
	stopAccepting := sync.OnceFunc(func() {
		listener.Close()
		os.Remove(dir)
	})
	return displayBridge{
		address: forwarded, stopAccepting: stopAccepting,
		close: sync.OnceFunc(func() {
			cancel()
			stopAccepting()
			serving.Wait()
		}),
	}, nil
}

func serveDisplay(ctx context.Context, listener *net.UnixListener, upstream string) {
	var streams sync.WaitGroup
	defer streams.Wait()
	for {
		client, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		streams.Go(func() { relayDisplay(ctx, client, upstream) })
	}
}

func relayDisplay(ctx context.Context, client *net.UnixConn, upstream string) {
	defer client.Close()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", upstream)
	if err != nil {
		return
	}
	server := conn.(*net.UnixConn)
	defer server.Close()
	stop := context.AfterFunc(ctx, func() {
		client.Close()
		server.Close()
	})
	defer stop()

	var sending sync.WaitGroup
	sending.Go(func() {
		io.Copy(server, client)
		server.CloseWrite()
	})
	io.Copy(client, server)
	client.CloseWrite()
	sending.Wait()
}
