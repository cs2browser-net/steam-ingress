package store_test

import (
	"bytes"
	"testing"

	"github.com/cs2browser-net/steam-ingress/store"
)

func TestAddressKey(t *testing.T) {
	key, ok := store.AddressKey("1.2.3.4:27015")
	if !ok || !bytes.Equal(key, []byte{1, 2, 3, 4, 0x69, 0x87}) {
		t.Fatalf("got %v %v", key, ok)
	}
	for _, bad := range []string{"1.2.3:27015", "1.2.3.4", "[::1]:27015", "1.2.3.256:1"} {
		if _, ok := store.AddressKey(bad); ok {
			t.Errorf("%s: expected rejection", bad)
		}
	}
}
