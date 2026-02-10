export interface ServerInfo {
    addr: string;
    gameport: number;
    steamid: string;
    name: string;
    appid: number;
    gamedir: string;
    version: string;
    product: string;
    region: number;
    players: number;
    max_players: number;
    bots: number;
    map: string;
    secure: boolean;
    dedicated: boolean;
    os: "l" | "w";
    gametype: string;
}

export interface ResponseInfo {
    servers: ServerInfo[];
}

export interface ServerListResponse {
    response: ResponseInfo;
}