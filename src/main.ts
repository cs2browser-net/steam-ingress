
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

    // const insertData = rows.map((r) => {
    //     const serverLocation = GetIPInfo(r.address.split(":")[0])

    //     return {
    //         ID: String(r.id),
    //         Address: r.address,
    //         Country: serverLocation.country,
    //         Latitute: serverLocation.latitude,
    //         Longitude: serverLocation.longitude,
    //         Status: r.status,
    //         LastUpdated: r.last_updated
    //     }
    // })

    // const chunkSize = 4096;
    // const chunks = [];
    // for (let i = 0; i < insertData.length; i += chunkSize) {
    //     chunks.push(insertData.slice(i, i + chunkSize));
    // }

    // var totalProcessed = 0;
    // for (const chunk of chunks) {
    //     await prisma.server.createMany({
    //         data: chunk,
    //         skipDuplicates: true
    //     });
    //     totalProcessed += chunk.length;
    //     console.log(`Inserted ${totalProcessed} / ${insertData.length} servers`);
    // }
}

(async () => {
    await updateCache();
    await requestServers(cfg);
    setInterval(async () => {
        await requestServers(cfg);
    }, 60000);
})()