import { serverCache } from "../cache"
import { GetIPInfo } from "./iplookup";
import { db } from "../db/drizzle";
import { server } from "../../generated/drizzle/schema";

export const AddServer = async (ip: string, status: number) => {
    serverCache.add(ip);

    const data = GetIPInfo(ip.split(":")[0])

    if (data.country == "cn" && ip.endsWith(":28000")) status = 9

    await db.insert(server).values({
        id: crypto.randomUUID(),
        address: ip,
        status: status,
        country: data.country,
        latitute: data.latitude,
        longitude: data.longitude
    })
}