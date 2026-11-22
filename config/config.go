package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Rule struct {
	Limit  int     `json:"limit"`
	Filter Ordered `json:"filter"`
}

type Config struct {
	SteamAPIKeys     []string `json:"steam_api_keys"`
	RetrievalFilters []Rule   `json:"retrieval_filters"`
	FilteredASN      []uint32 `json:"filtered_asn"`
	Expressions      Ordered  `json:"expressions"`
}

type Ordered []Entry

type Entry struct {
	Key   string
	Value json.RawMessage
}

func (o *Ordered) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return errors.New("expected a JSON object")
	}
	*o = (*o)[:0]
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return err
		}
		*o = append(*o, Entry{Key: tok.(string), Value: v})
	}
	return nil
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(cfg.SteamAPIKeys) == 0 {
		return nil, errors.New("steam_api_keys must contain at least one key")
	}
	return &cfg, nil
}
