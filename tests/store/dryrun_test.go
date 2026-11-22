package store_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cs2browser-net/steam-ingress/geoip"
	"github.com/cs2browser-net/steam-ingress/store"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	fn()
	os.Stdout = old
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestDryRunPrintsInsteadOfWriting(t *testing.T) {
	st := store.New(nil, nil, "servers", geoip.New("127.0.0.1:1", 1, 50*time.Millisecond), true)
	key, _ := store.AddressKey("1.2.3.4:27015")

	out := captureStdout(t, func() {
		for range 2 {
			if err := st.AddServer(context.Background(), "1.2.3.4:27015", 2, key); err != nil {
				t.Error(err)
			}
		}
	})

	if n := strings.Count(out, "DRY RUN sql: INSERT INTO"); n != 1 {
		t.Errorf("want 1 INSERT, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, "DRY RUN redis: SADD servers 010203046987"); n != 1 {
		t.Errorf("want 1 SADD, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "1.2.3.4:27015 2 Unknown 0 0") {
		t.Errorf("INSERT args missing:\n%s", out)
	}
}
