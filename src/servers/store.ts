import { ResultSetHeader } from "mysql2";
import { serverCache } from "../cache"
import { DoQuery } from "../db/mysql";
import { prisma } from "../db/prisma";
import { GetIPInfo } from "./iplookup";

export const AddServer = async (ip: string, status: number) => {
    serverCache.add(ip);

    const data = GetIPInfo(ip.split(":")[0])

    if (data.country == "cn" && ip.endsWith(":28000")) status = 9

    const result = await DoQuery("insert into `ip_list` (`address`, `country`, `lat`, `lon`, `status`) values (?, ?, ?, ?, ?)", [ip, data.country, data.latitude, data.longitude, status])

    // @ts-expect-error
    const res = result as ResultSetHeader

    await prisma.server.create({
        data: {
            ID: String(res.insertId),
            Address: ip,
            Status: status,
            Country: data.country,
            Latitute: data.latitude,
            Longitude: data.longitude,
        }
    })
}