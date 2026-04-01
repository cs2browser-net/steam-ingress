import 'dotenv/config'
import Redis from "ioredis";

export const cacheClient = new Redis(process.env.CACHE_URL!);
export const serversCacheKey = process.env.SERVERS_KEY!;