package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cs2browser-net/steam-ingress/config"
	"github.com/cs2browser-net/steam-ingress/geoip"
	"github.com/cs2browser-net/steam-ingress/logger"
	"github.com/cs2browser-net/steam-ingress/rules"
	"github.com/cs2browser-net/steam-ingress/steam"
	"github.com/cs2browser-net/steam-ingress/store"
)

const serverListURL = "https://api.steampowered.com/IGameServersService/GetServerList/v1/"

var regions = []string{"0", "1", "2", "3", "4", "5", "6", "7", "255"}

type Fetcher struct {
	cfg        *config.Config
	filters    []string
	rules      *rules.Set
	store      *store.Store
	geo        *geoip.Client
	http       *http.Client
	tokenIndex int
}

func New(cfg *config.Config, st *store.Store, geo *geoip.Client) (*Fetcher, error) {
	f := &Fetcher{cfg: cfg, store: st, geo: geo, http: &http.Client{Timeout: 2 * time.Minute}}

	for _, rule := range cfg.RetrievalFilters {
		filter, err := steam.BuildFilters(rule.Filter)
		if err != nil {
			return nil, err
		}
		f.filters = append(f.filters, filter)
	}

	var err error
	if f.rules, err = rules.Compile(cfg.Expressions); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Fetcher) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		f.requestServers(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (f *Fetcher) requestServers(ctx context.Context) {
	f.tokenIndex = (f.tokenIndex + 1) % len(f.cfg.SteamAPIKeys)
	token := f.cfg.SteamAPIKeys[f.tokenIndex]

	for _, region := range regions {
		logger.Printf("Fetching servers for region '%s'...", region)
		for i, rule := range f.cfg.RetrievalFilters {
			if ctx.Err() != nil {
				return
			}
			filter := strings.ReplaceAll(f.filters[i], "{{REGION}}", region)
			if err := f.filterRequest(ctx, token, rule.Limit, filter); err != nil {
				logger.Printf("Request for filter %s failed: %v", filter, err)
			}
		}
	}
}

func (f *Fetcher) fetchServers(ctx context.Context, token string, limit int, filter string) ([]steam.ServerInfo, error) {
	query := url.Values{"key": {token}, "limit": {strconv.Itoa(limit)}, "filter": {filter}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverListURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	resp, err := f.http.Do(req)
	if err != nil {
		if uerr, ok := errors.AsType[*url.Error](err); ok {
			err = uerr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam API returned %s", resp.Status)
	}

	var body struct {
		Response struct {
			Servers []steam.ServerInfo `json:"servers"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode steam response: %w", err)
	}
	return body.Response.Servers, nil
}

func (f *Fetcher) filterRequest(ctx context.Context, token string, limit int, filter string) error {
	servers, err := f.fetchServers(ctx, token, limit, filter)
	if err != nil || servers == nil {
		return err
	}

	serversPerIP := make(map[string]int, len(servers))
	ips := make([]string, 0, len(servers))
	for _, s := range servers {
		ip, port, _ := strings.Cut(s.Addr, ":")
		if ip == "" || port == "" {
			continue
		}
		if serversPerIP[ip] == 0 {
			ips = append(ips, ip)
		}
		serversPerIP[ip]++
	}

	asns := f.geo.ASNBatch(ctx, ips)
	asnByIP := make(map[string]uint32, len(ips))
	for i, ip := range ips {
		asnByIP[ip] = asns[i]
	}

	for i := range servers {
		s := &servers[i]
		key, ok := store.AddressKey(s.Addr)
		if !ok {
			continue
		}

		ip, _, _ := strings.Cut(s.Addr, ":")
		status := 8
		if !slices.Contains(f.cfg.FilteredASN, asnByIP[ip]) && serversPerIP[ip] <= 15 {
			s.Tags = strings.Split(s.GameType, ",")
			status = f.rules.Evaluate(s)
		}

		if err := f.store.AddServer(ctx, s.Addr, status, key); err != nil {
			return fmt.Errorf("add server %s: %w", s.Addr, err)
		}
	}

	logger.Printf("Fetched '%d' servers from the API.", len(servers))
	return nil
}
