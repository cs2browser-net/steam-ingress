
import { config } from "dotenv";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { DoQuery } from "./db/mysql";
import { serverCache } from './cache'
import { requestServers } from './servers/fetcher'

config()

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    const rows = await DoQuery("select id, address, status, last_updated from `ip_list`")

    for (const row of rows) {
        serverCache.add(row.address);
    }
}

(async () => {
    await updateCache();
    await requestServers(cfg);
    setInterval(async () => {
        await requestServers(cfg);
    }, 60000);
})()