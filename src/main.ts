
import "dotenv/config";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { db } from "./db/drizzle";
import { server } from "../generated/drizzle/schema";
import { requestServers } from "./servers/fetcher";
import { cacheClient, serversCacheKey } from "./cache/redis";
import { AddressToBuffer, type ReusableAddressBuffer } from './utils/ipPreprocessor';
import { log } from './utils/console'

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    log("Updating cache...");
    const servers = await db.select({ Address: server.address }).from(server);
    log(`Fetched ${servers.length} servers from database`);

    log("Updating cache...");
    const cacheQueue: ReusableAddressBuffer[] = [];

    const flushCacheQueue = async () => {
        if (cacheQueue.length === 0) return;

        const queue = cacheQueue.splice(0, cacheQueue.length);
        const queueBuffers = queue.map((entry) => entry.buffer);

        try {
            await cacheClient.sadd(serversCacheKey, queueBuffers);
        } finally {
            for (const entry of queue) {
                entry.release();
            }
        }
    };

    for (const row of servers) {
        let buffer = AddressToBuffer(row.Address);
        if (buffer == null) continue;

        cacheQueue.push(buffer);

        if (cacheQueue.length >= 1024) {
            await flushCacheQueue();
        }
    }

    await flushCacheQueue();
    log(`Updated cache.`);
}

(async () => {
    await updateCache();
    await requestServers(cfg);
    setInterval(async () => {
        await requestServers(cfg);
    }, 60000);
})()