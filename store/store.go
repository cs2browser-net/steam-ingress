package store

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cs2browser-net/steam-ingress/geoip"
	"github.com/cs2browser-net/steam-ingress/logger"
)

const (
	cacheBatchSize = 1024

	selectAddresses = `SELECT "Address" FROM "Server"`
	insertServer    = `INSERT INTO "Server" ("ID", "Address", "Status", "Country", "Latitute", "Longitude")
 VALUES ($1, $2, $3, $4, $5, $6)
 ON CONFLICT ("Address") DO NOTHING`
)

type Store struct {
	db       *pgxpool.Pool
	cache    *redis.Client
	cacheKey string
	geo      *geoip.Client

	dryRun bool
	seen   map[string]struct{}
}

func New(db *pgxpool.Pool, cache *redis.Client, cacheKey string, geo *geoip.Client, dryRun bool) *Store {
	return &Store{db: db, cache: cache, cacheKey: cacheKey, geo: geo, dryRun: dryRun, seen: map[string]struct{}{}}
}

func AddressKey(addr string) ([]byte, bool) {
	ap, err := netip.ParseAddrPort(addr)
	if err != nil || !ap.Addr().Is4() {
		return nil, false
	}
	ip := ap.Addr().As4()
	key := make([]byte, 6)
	copy(key, ip[:])
	binary.BigEndian.PutUint16(key[4:], ap.Port())
	return key, true
}

func (s *Store) WarmCache(ctx context.Context) error {
	rows, err := s.db.Query(ctx, selectAddresses)
	if err != nil {
		return err
	}
	addrs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	logger.Printf("Fetched %d servers from database", len(addrs))

	batch := make([]any, 0, cacheBatchSize)
	for _, addr := range addrs {
		key, ok := AddressKey(addr)
		if !ok {
			continue
		}
		if s.dryRun {
			s.seen[string(key)] = struct{}{}
			continue
		}
		batch = append(batch, key)
		if len(batch) == cacheBatchSize {
			if err := s.cache.SAdd(ctx, s.cacheKey, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if s.dryRun {
		logger.Printf("DRY RUN redis: SADD %s <%d known addresses, 6-byte keys, batches of %d>", s.cacheKey, len(s.seen), cacheBatchSize)
		return nil
	}
	if len(batch) > 0 {
		return s.cache.SAdd(ctx, s.cacheKey, batch...).Err()
	}
	return nil
}

func (s *Store) AddServer(ctx context.Context, addr string, status int, key []byte) error {
	if s.dryRun {
		if _, dup := s.seen[string(key)]; dup {
			return nil
		}
		s.seen[string(key)] = struct{}{}
		logger.Printf("DRY RUN redis: SADD %s %x", s.cacheKey, key)
	} else if added, err := s.cache.SAdd(ctx, s.cacheKey, key).Result(); err != nil || added == 0 {
		return err
	}

	city := s.geo.City(ctx, addr)
	if city.Country == "cn" && strings.HasSuffix(addr, ":28000") {
		status = 9
	}

	args := []any{uuid.NewString(), addr, status, city.Country, city.Latitude, city.Longitude}
	if s.dryRun {
		logger.Printf("DRY RUN sql: %s\n  args: %v", insertServer, args)
		return nil
	}

	if _, err := s.db.Exec(ctx, insertServer, args...); err != nil {
		s.cache.SRem(context.WithoutCancel(ctx), s.cacheKey, key)
		return err
	}
	return nil
}
