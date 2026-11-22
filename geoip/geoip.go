package geoip

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cs2browser-net/steam-ingress/logger"
)

// Wire protocol of the GeoIP service:
//
//	request:  [query u8][ipLen u8][ip]                          (single)
//	          [3 u8][count u16]([query u8][ipLen u8][ip])*     (batch)
//	response: [status u8][payloadLen u32][payload]
const (
	queryASN   byte = 0
	queryCity  byte = 1
	queryBatch byte = 3

	statusOK byte = 0

	maxFrameSize = 16 << 20
	maxBatchSize = 1024
)

type geoResponse struct {
	status  byte
	payload []byte
}

type City struct {
	Country   string
	Latitude  float64
	Longitude float64
}

var Unknown = City{Country: "Unknown"}

type Client struct {
	addr    string
	timeout time.Duration
	pool    chan *geoConn
}

type geoConn struct{ conn net.Conn }

func New(addr string, size int, timeout time.Duration) *Client {
	c := &Client{addr: addr, timeout: timeout, pool: make(chan *geoConn, size)}
	for range size {
		c.pool <- &geoConn{}
	}
	return c
}

func (c *Client) request(ctx context.Context, frame []byte) (geoResponse, error) {
	var gc *geoConn
	select {
	case gc = <-c.pool:
	case <-ctx.Done():
		return geoResponse{}, ctx.Err()
	}
	defer func() { c.pool <- gc }()

	for {
		reused := gc.conn != nil
		resp, err := gc.roundTrip(c.addr, c.timeout, frame)
		if err == nil {
			return resp, nil
		}
		if gc.conn != nil {
			gc.conn.Close()
			gc.conn = nil
		}
		if !reused {
			return geoResponse{}, err
		}
	}
}

func (g *geoConn) roundTrip(addr string, timeout time.Duration, frame []byte) (geoResponse, error) {
	if g.conn == nil {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return geoResponse{}, err
		}
		g.conn = conn
	}
	if err := g.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return geoResponse{}, err
	}
	if _, err := g.conn.Write(frame); err != nil {
		return geoResponse{}, err
	}

	var header [5]byte
	if _, err := io.ReadFull(g.conn, header[:]); err != nil {
		return geoResponse{}, err
	}
	n := binary.BigEndian.Uint32(header[1:])
	if n > maxFrameSize {
		return geoResponse{}, fmt.Errorf("GeoIP payload too large: %d bytes", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(g.conn, payload); err != nil {
		return geoResponse{}, err
	}
	return geoResponse{status: header[0], payload: payload}, nil
}

func (c *Client) City(ctx context.Context, ip string) City {
	city, err := c.city(ctx, ip)
	if err != nil {
		logger.Printf("GeoIP city lookup for %s failed: %v", ip, err)
		return Unknown
	}
	return city
}

func (c *Client) city(ctx context.Context, ip string) (City, error) {
	ip = normalizeIP(ip)
	if len(ip) == 0 || len(ip) > 255 {
		return City{}, errors.New("IP length must be between 1 and 255 bytes")
	}
	resp, err := c.request(ctx, append([]byte{queryCity, byte(len(ip))}, ip...))
	if err != nil {
		return City{}, err
	}
	if resp.status != statusOK {
		return City{}, fmt.Errorf("status %d", resp.status)
	}
	return parseCity(resp.payload)
}

func (c *Client) ASNBatch(ctx context.Context, ips []string) []uint32 {
	out := make([]uint32, 0, len(ips))
	for chunk := range slices.Chunk(ips, maxBatchSize) {
		asns := make([]uint32, len(chunk))
		if err := c.asnChunk(ctx, chunk, asns); err != nil {
			logger.Printf("GeoIP ASN batch lookup of %d IPs failed: %v", len(chunk), err)
		}
		out = append(out, asns...)
	}
	return out
}

func (c *Client) asnChunk(ctx context.Context, ips []string, asns []uint32) error {
	frame := []byte{queryBatch, 0, 0}
	binary.BigEndian.PutUint16(frame[1:], uint16(len(ips)))
	for _, ip := range ips {
		ip = normalizeIP(ip)
		if len(ip) == 0 || len(ip) > 255 {
			return errors.New("IP length must be between 1 and 255 bytes")
		}
		frame = append(append(frame, queryASN, byte(len(ip))), ip...)
	}

	resp, err := c.request(ctx, frame)
	if err != nil {
		return err
	}
	if resp.status != statusOK {
		return fmt.Errorf("status %d", resp.status)
	}
	items, err := parseBatch(resp.payload, len(ips))
	if err != nil {
		return err
	}
	for i, item := range items {
		if item.status != statusOK {
			continue
		}
		if asn, err := parseASN(item.payload); err == nil {
			asns[i] = asn
		}
	}
	return nil
}

func normalizeIP(input string) string {
	if strings.HasPrefix(input, "[") {
		if end := strings.Index(input, "]"); end > 1 {
			return input[1:end]
		}
	}
	if first := strings.Index(input, ":"); first != -1 && first == strings.LastIndex(input, ":") {
		return input[:first]
	}
	return input
}

func parseCity(payload []byte) (City, error) {
	r := payloadReader{b: payload}
	country, hasCountry := r.optString()
	for range 4 {
		r.optString()
	}
	lat, _ := r.optFloat64()
	lon, _ := r.optFloat64()
	r.optString()
	if err := r.done(); err != nil {
		return City{}, err
	}

	city := City{Country: "Unknown", Latitude: finiteOrZero(lat), Longitude: finiteOrZero(lon)}
	if hasCountry && country != "" {
		city.Country = strings.ToLower(country)
	}
	return city, nil
}

func parseASN(payload []byte) (uint32, error) {
	r := payloadReader{b: payload}
	asn, _ := r.optUint32()
	r.optString()
	return asn, r.done()
}

func parseBatch(payload []byte, expected int) ([]geoResponse, error) {
	if len(payload) < 2 {
		return nil, errors.New("unexpected end of batch payload")
	}
	count := int(binary.BigEndian.Uint16(payload))
	if count != expected {
		return nil, fmt.Errorf("batch response count mismatch: expected %d, got %d", expected, count)
	}

	out := make([]geoResponse, count)
	rest := payload[2:]
	for i := range out {
		if len(rest) < 5 {
			return nil, errors.New("unexpected end of batch payload")
		}
		n := binary.BigEndian.Uint32(rest[1:5])
		if uint64(n) > uint64(len(rest)-5) {
			return nil, errors.New("unexpected end of batch item payload")
		}
		out[i] = geoResponse{status: rest[0], payload: rest[5 : 5+n]}
		rest = rest[5+n:]
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("unexpected trailing bytes in batch payload: %d", len(rest))
	}
	return out, nil
}

func finiteOrZero(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

type payloadReader struct {
	b   []byte
	err error
}

func (r *payloadReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n > len(r.b) {
		r.err = errors.New("unexpected end of payload")
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *payloadReader) present() bool {
	b := r.take(1)
	if r.err != nil {
		return false
	}
	switch b[0] {
	case 0:
		return false
	case 1:
		return true
	default:
		r.err = fmt.Errorf("invalid presence flag: %d", b[0])
		return false
	}
}

func (r *payloadReader) optString() (string, bool) {
	if !r.present() {
		return "", false
	}
	n := r.take(2)
	if r.err != nil {
		return "", false
	}
	s := r.take(int(binary.BigEndian.Uint16(n)))
	if r.err != nil {
		return "", false
	}
	if !utf8.Valid(s) {
		r.err = errors.New("invalid UTF-8 string")
		return "", false
	}
	return string(s), true
}

func (r *payloadReader) optUint32() (uint32, bool) {
	if !r.present() {
		return 0, false
	}
	b := r.take(4)
	if r.err != nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}

func (r *payloadReader) optFloat64() (float64, bool) {
	if !r.present() {
		return 0, false
	}
	b := r.take(8)
	if r.err != nil {
		return 0, false
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b)), true
}

func (r *payloadReader) done() error {
	if r.err != nil {
		return r.err
	}
	if len(r.b) != 0 {
		return fmt.Errorf("unexpected trailing bytes: %d", len(r.b))
	}
	return nil
}
