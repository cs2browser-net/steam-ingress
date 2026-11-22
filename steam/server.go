package steam

type ServerInfo struct {
	Addr       string   `json:"addr" expr:"addr"`
	GamePort   int      `json:"gameport" expr:"gameport"`
	SteamID    string   `json:"steamid" expr:"steamid"`
	Name       string   `json:"name" expr:"name"`
	AppID      int      `json:"appid" expr:"appid"`
	GameDir    string   `json:"gamedir" expr:"gamedir"`
	Version    string   `json:"version" expr:"version"`
	Product    string   `json:"product" expr:"product"`
	Region     int      `json:"region" expr:"region"`
	Players    int      `json:"players" expr:"players"`
	MaxPlayers int      `json:"max_players" expr:"max_players"`
	Bots       int      `json:"bots" expr:"bots"`
	Map        string   `json:"map" expr:"map"`
	Secure     bool     `json:"secure" expr:"secure"`
	Dedicated  bool     `json:"dedicated" expr:"dedicated"`
	OS         string   `json:"os" expr:"os"`
	GameType   string   `json:"gametype" expr:"gametype"`
	Tags       []string `json:"-" expr:"tags"`
}
