package geoip_test

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"net"
	"testing"
	"time"

	"github.com/cs2browser-net/steam-ingress/geoip"
)

const (
	queryCity  = 1
	queryBatch = 3
	statusOK   = 0
)

func optString(s string) []byte {
	b := []byte{1, 0, 0}
	binary.BigEndian.PutUint16(b[1:], uint16(len(s)))
	return append(b, s...)
}

func optF64(f float64) []byte {
	b := make([]byte, 9)
	b[0] = 1
	binary.BigEndian.PutUint64(b[1:], math.Float64bits(f))
	return b
}

func frame(status byte, payload []byte) []byte {
	b := []byte{status, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[1:], uint32(len(payload)))
	return append(b, payload...)
}

func cityPayload(country string, lat, lon float64) []byte {
	p := optString(country)
	p = append(p, 0, 0, 0, 0) // four absent strings
	p = append(p, optF64(lat)...)
	p = append(p, optF64(lon)...)
	return append(p, 0)
}

func asnPayload(asn uint32) []byte {
	p := []byte{1, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(p[1:], asn)
	return append(p, 0)
}

// serve starts a fake GeoIP server. reply receives the query kind and the requested IPs and returns
// the raw response frame. The connection is closed after each reply to exercise reconnection.
func serve(t *testing.T, reply func(kind byte, ips []string) []byte) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	readIP := func(conn net.Conn) string {
		var n [1]byte
		io.ReadFull(conn, n[:])
		ip := make([]byte, n[0])
		io.ReadFull(conn, ip)
		return string(ip)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				var kind [1]byte
				if _, err := io.ReadFull(conn, kind[:]); err != nil {
					return
				}
				var ips []string
				if kind[0] == queryBatch {
					var count [2]byte
					io.ReadFull(conn, count[:])
					for range binary.BigEndian.Uint16(count[:]) {
						io.ReadFull(conn, make([]byte, 1)) // per-item query byte
						ips = append(ips, readIP(conn))
					}
				} else {
					ips = append(ips, readIP(conn))
				}
				conn.Write(reply(kind[0], ips))
			}()
		}
	}()
	return ln.Addr().String()
}

func TestClient(t *testing.T) {
	seen := make(chan string, 8)
	addr := serve(t, func(kind byte, ips []string) []byte {
		if kind == queryCity {
			seen <- ips[0]
			return frame(statusOK, cityPayload("DE", 51.5, math.NaN()))
		}
		payload := binary.BigEndian.AppendUint16(nil, uint16(len(ips)))
		for i := range ips {
			payload = append(payload, frame(statusOK, asnPayload(100+uint32(i)))...)
		}
		return frame(statusOK, payload)
	})
	c := geoip.New(addr, 1, time.Second)
	ctx := context.Background()

	for _, in := range []string{"1.2.3.4:27015", "[::1]:27015"} { // the second call runs on a connection the server already closed
		city := c.City(ctx, in)
		if city != (geoip.City{Country: "de", Latitude: 51.5, Longitude: 0}) {
			t.Fatalf("city: %+v", city)
		}
	}
	if a, b := <-seen, <-seen; a != "1.2.3.4" || b != "::1" {
		t.Fatalf("port and brackets must be stripped before the query, got %q and %q", a, b)
	}

	asns := c.ASNBatch(ctx, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
	if len(asns) != 3 || asns[0] != 100 || asns[2] != 102 {
		t.Fatalf("asns: %v", asns)
	}
}

func TestUnavailableFallsBack(t *testing.T) {
	c := geoip.New("127.0.0.1:1", 1, 200*time.Millisecond)
	if city := c.City(context.Background(), "1.2.3.4"); city != geoip.Unknown {
		t.Fatalf("city: %+v", city)
	}
	if asns := c.ASNBatch(context.Background(), []string{"1.1.1.1"}); len(asns) != 1 || asns[0] != 0 {
		t.Fatalf("asns: %v", asns)
	}
}

func TestMalformedPayloadsFallBack(t *testing.T) {
	replies := map[string][]byte{
		"bad presence flag":   frame(statusOK, []byte{2}),
		"trailing bytes":      frame(statusOK, append(cityPayload("DE", 1, 2), 0xff)),
		"truncated batch":     frame(statusOK, []byte{0, 1, 0, 0, 0, 0, 9}),
		"non-OK status":       frame(2, nil),
		"batch count differs": frame(statusOK, []byte{0, 5}),
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			c := geoip.New(serve(t, func(byte, []string) []byte { return reply }), 1, time.Second)
			if city := c.City(context.Background(), "1.2.3.4"); city != geoip.Unknown {
				t.Errorf("city: %+v", city)
			}
			if asns := c.ASNBatch(context.Background(), []string{"1.1.1.1"}); len(asns) != 1 || asns[0] != 0 {
				t.Errorf("asns: %v", asns)
			}
		})
	}
}
