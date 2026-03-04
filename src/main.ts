
import { config } from "dotenv";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { serverCache } from './cache'
import { requestServers } from './servers/fetcher'
import { prisma } from "./db/prisma";

config()

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    const servers = await prisma.server.findMany({
        select: {
            Address: true
        }
    })

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