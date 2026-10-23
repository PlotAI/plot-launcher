package protocol

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeShutsDownWithContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	handler := NewHandler(&fakeOpener{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := NewServer(handler)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, server)
	}()

	response, err := http.Get(fmt.Sprintf("http://%s/", listener.Addr()))
	if err != nil {
		t.Fatalf("GET root: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not shut down")
	}
}

func TestListenRejectsInvalidPort(t *testing.T) {
	if _, err := Listen("127.0.0.1", 0); err == nil {
		t.Fatal("Listen with port 0 returned no error")
	}
	if _, err := Listen("127.0.0.1", 70000); err == nil {
		t.Fatal("Listen with an out-of-range port returned no error")
	}
}
