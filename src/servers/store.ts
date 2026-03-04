import { serverCache } from "../cache"
import { prisma } from "../db/prisma";
import { GetIPInfo } from "./iplookup";

export const AddServer = async (ip: string, status: number) => {
    serverCache.add(ip);

    const data = GetIPInfo(ip.split(":")[0])

    if (data.country == "cn" && ip.endsWith(":28000")) status = 9

    await prisma.server.create({
        data: {
            Address: ip,
            Status: status,
            Country: data.country,
            Latitute: data.latitude,
            Longitude: data.longitude,
        }
    })
}