import { drizzle } from 'drizzle-orm/node-postgres';
import * as schema from '../../generated/drizzle/schema';
import { config } from "dotenv";

config()

export const db = drizzle({ connection: process.env.DATABASE_URL!, schema });
