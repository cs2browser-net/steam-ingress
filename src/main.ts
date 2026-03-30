
import { config } from "dotenv";
import { readFileSync } from "fs";
import type { Config } from "./types/config";
import { serverCache } from './cache'
import { db } from "./db/drizzle";
import { server } from "../generated/drizzle/schema";

config()

const cfg: Config = JSON.parse(readFileSync("data/config.json").toString());

const updateCache = async () => {
    var start = Date.now();
    const servers = await db.select({ Address: server.address }).from(server);

    for (const row of servers) {
        serverCache.add(row.Address);
    }
    var end = Date.now();
    console.log(`Cache updated with ${servers.length} entries in ${(end - start) / 1000} seconds.`);
}

(async () => {
    await updateCache();
    // await requestServers(cfg);
    // setInterval(async () => {
    //     await requestServers(cfg);
    // }, 60000);
})()