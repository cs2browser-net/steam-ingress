
import "dotenv/config";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { serverCache } from './cache'
import { db } from "./db/drizzle";
import { server } from "../generated/drizzle/schema";
import { requestServers } from "./servers/fetcher";

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    const servers = await db.select({ Address: server.address }).from(server);

    for (const row of servers) {
        serverCache.add(row.Address);
    }
}

(async () => {
    await updateCache();
    await requestServers(cfg);
    setInterval(async () => {
        await requestServers(cfg);
    }, 60000);
})()