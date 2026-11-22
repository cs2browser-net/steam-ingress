# steam-ingress

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Build Status](https://img.shields.io/github/actions/workflow/status/cs2browser-net/steam-ingress/build.yml?branch=master)](https://github.com/cs2browser-net/steam-ingress/actions)
[![Release](https://img.shields.io/github/v/release/cs2browser-net/steam-ingress?include_prereleases)](https://github.com/cs2browser-net/steam-ingress/releases)

Polls the Steam server list for CS2 servers, classifies each new one, and stores it in PostgreSQL.

Built in Go, ships as a single static binary. Every minute it queries Steam's `IGameServersService/GetServerList` for each region and each retrieval filter in your config. Addresses it has already seen are skipped through a Redis set. A new server is looked up in a GeoIP service, given a status by your config's rules, and inserted into the `Server` table.

## Install

Grab an archive from the [latest release](https://github.com/cs2browser-net/steam-ingress/releases/latest):

| Archive                                                                                                                          | Use when            |
| -------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| [`steam-ingress-win-x64.zip`](https://github.com/cs2browser-net/steam-ingress/releases/latest/download/steam-ingress-win-x64.zip)     | Windows             |
| [`steam-ingress-linux-x64.zip`](https://github.com/cs2browser-net/steam-ingress/releases/latest/download/steam-ingress-linux-x64.zip) | Linux               |
| [`steam-ingress-linux-arm64.zip`](https://github.com/cs2browser-net/steam-ingress/releases/latest/download/steam-ingress-linux-arm64.zip) | Linux on ARM        |
| [`steam-ingress-osx-arm64.zip`](https://github.com/cs2browser-net/steam-ingress/releases/latest/download/steam-ingress-osx-arm64.zip) | macOS Apple Silicon |

Those links always resolve to the newest stable release. On Linux and macOS, `chmod +x steam-ingress` after unzipping.

Or build from source with Go installed:

```bash
go build -trimpath -ldflags "-s -w" -o steam-ingress .
```

## Requirements

- PostgreSQL, with the tables from [`schema.sql`](schema.sql). steam-ingress only writes to `"Server"` and needs its unique `"Server_Address_key"` index.
- Redis.
- A GeoIP service reachable over TCP (see `GEOIP_*` below).
- At least one Steam Web API key.

## Configuration

Copy [`.env.example`](.env.example) to `.env` (or set real environment variables, which take precedence) and [`data/config.example.json`](data/config.example.json) to `data/config.json`.

| Variable                   | Default     | Purpose                                      |
| -------------------------- | ----------- | -------------------------------------------- |
| `DATABASE_URL`             |             | PostgreSQL connection string                 |
| `CACHE_URL`                |             | Redis connection URL                         |
| `SERVERS_KEY`              |             | Redis set holding the addresses already seen |
| `GEOIP_HOST`               | `127.0.0.1` | GeoIP service host                           |
| `GEOIP_PORT`               | `8081`      | GeoIP service port                           |
| `GEOIP_POOL_SIZE`          | `12`        | Concurrent GeoIP connections                 |
| `GEOIP_REQUEST_TIMEOUT_MS` | `2500`      | Per-request GeoIP timeout                    |

`data/config.json`:

```json
{
    "steam_api_keys": ["KEY"],
    "retrieval_filters": [
        {
            "limit": 10000,
            "filter": {
                "appid": 730,
                "region": "{{REGION}}",
                "gamedir": "csgo",
                "nor": { "name_match": ["Valve*"], "gametype": "empty" }
            }
        }
    ],
    "filtered_asn": [],
    "expressions": {
        "max_players > 64": 2
    }
}
```

- **`steam_api_keys`**: one key is used per poll, rotating through the list.
- **`retrieval_filters`**: one Steam query per entry and region. `filter` becomes Steam's backslash-separated filter string in the order you write it. An array repeats its key, `nor` / `nand` nest, and `{{REGION}}` is replaced with each of Steam's regions (`0`-`7`, `255`).
- **`filtered_asn`**: servers hosted in these ASNs get status `8`, as do servers on an IP that runs more than 15 of them.
- **`expressions`**: `"expression": status`, evaluated in order against each server. The first one that is true decides the status, and if none match it is `0`. A Chinese server on port `28000` always gets status `9`.

Expressions use [expr-lang](https://expr-lang.org). The variables are `addr`, `gameport`, `steamid`, `name`, `appid`, `gamedir`, `version`, `product`, `region`, `players`, `max_players`, `bots`, `map`, `secure`, `dedicated`, `os`, `gametype` and `tags` (the comma-split `gametype`). Extra functions are `number(x)`, `string(x)`, `startsWith(a, b)` and `includes(list, x)`. Expressions are type-checked at startup, so a typo stops the service instead of failing on every poll.

`data/config.json` and `.env` hold credentials. They are git-ignored, keep it that way.

## Usage

```bash
./steam-ingress
```

On start it loads every address from the `Server` table into Redis, then polls until `Ctrl+C` / `SIGTERM`, which cancels the request in flight and exits.

## Layout

| Package        | Purpose                                                                                                   |
| -------------- | --------------------------------------------------------------------------------------------------------- |
| `main.go`      | Entry point. Reads the environment, wires the packages together and runs the poller.                      |
| `config`       | Loads and validates `data/config.json`. `Ordered` is a JSON object that keeps key order.                  |
| `steam`        | `ServerInfo` (one Steam server, and the variables available to expressions) and the Steam filter builder. |
| `rules`        | Compiles the config expressions and evaluates them against a server.                                      |
| `geoip`        | Client for the GeoIP service: pooled TCP connections, country/coordinates and batched ASN lookups.        |
| `store`        | PostgreSQL inserts and the Redis "already seen" set.                                                      |
| `fetcher`      | The poll loop: queries Steam, classifies each server and hands new ones to `store`.                       |
| `logger`       | Timestamped logging.                                                                                      |
| `tests`        | Tests, one directory per package, using only each package's exported API.                                 |

## Development

```bash
go vet ./...
go test ./...
```

Versions come from [GitVersion](GitVersion.yml): every push to `master` is tagged and released by the build workflow. Add `+semver: minor` or `+semver: major` to a commit message to bump past a patch.

## License

[GPL-3.0](LICENSE)
