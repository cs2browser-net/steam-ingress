
import "dotenv/config";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { db } from "./db/drizzle";
import { server } from "../generated/drizzle/schema";
import { requestServers } from "./servers/fetcher";
import { cacheClient, serversCacheKey } from "./cache/redis";
import { AddressToBuffer } from './utils/ipPreprocessor';
import { log } from './utils/console'

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    log("Updating cache...");
    const servers = await db.select({ Address: server.address }).from(server);
    log(`Fetched ${servers.length} servers from database`);

    log("Updating cache...");
    const cacheQueue = [];
    for (const row of servers) {
        let buffer = AddressToBuffer(row.Address);
        if (buffer == null) continue;

        cacheQueue.push(buffer);

        if (cacheQueue.length >= 1024) {
            await cacheClient.sadd(serversCacheKey, cacheQueue);
            cacheQueue.length = 0;
        }
    }

    if (cacheQueue.length > 0) {
        await cacheClient.sadd(serversCacheKey, cacheQueue);
    }
    log(`Updated cache.`);
}

(async () => {
    await updateCache();
    await requestServers(cfg);
    setInterval(async () => {
        await requestServers(cfg);
    }, 60000);
})()