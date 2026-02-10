import { CreateDatabase } from "../db/mysql";
import type { Config, Rules } from "../types/config"
import type { ServerListResponse } from "../types/SteamAPI"
import { buildFilters, evaluateExpression } from './filters'
import http from "../http"
import { serverCache } from "../cache";
import { GetIPASN } from "./iplookup";
import { AddServer } from "./store"

const regions = ["0", "1", "2", "3", "4", "5", "6", "7", "255"]
const serverRequestURL = `https://api.steampowered.com/IGameServersService/GetServerList/v1/?key={{STEAM_KEY}}&limit={{LIMIT}}&filter=`;

let tokenIndex = 0

async function FilterRequest(cfg: Config, url: string) {
    const fullResponse = await http.get(url)
    const res: ServerListResponse = fullResponse.data

    const { servers } = res.response
    if (typeof servers != "object") return;

    for (const server of servers) {
        if (serverCache.has(server.addr)) continue;

        const { asn } = GetIPASN(server.addr.split(":")[0])

        if (cfg.filtered_asn.includes(asn)) await AddServer(server.addr, 8)
        else {
            let status = 0

            for (const expression of Object.keys(cfg.expressions)) {
                status = evaluateExpression(cfg, expression, server);
                if (status != 0) break;
            }

            await AddServer(server.addr, status)
        }
    }
    console.log(`Fetched '${servers.length}' servers from the API.`)
}

async function RequestInformations(region: string, token: string, cfg: Config) {
    console.log(`Fetching servers for region '${region}'...`)

    for (const rule of cfg.retrieval_filters) {
        let filters = buildFilters(rule.filter)
        const requestURL = (serverRequestURL + filters).replace("{{STEAM_KEY}}", token).replace("{{LIMIT}}", rule.limit.toString()).replace("{{REGION}}", region)
        try {
            await FilterRequest(cfg, requestURL)
        } catch (e) { }
    }
}

export async function requestServers(cfg: Config) {
    await CreateDatabase();

    if (tokenIndex + 1 == cfg.steam_api_keys.length) tokenIndex = 0;
    else tokenIndex++;

    let token = cfg.steam_api_keys[tokenIndex]

    for (const region of regions) {
        await RequestInformations(region, token, cfg)
    }
}