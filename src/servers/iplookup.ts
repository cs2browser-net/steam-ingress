import { readFileSync } from "fs";
import { AsnResponse, CityResponse, Reader } from "maxmind";

const buffer = readFileSync("data/dbip-city-ipv4.mmdb");
const lookup = new Reader<CityResponse>(buffer);

const buffer2 = readFileSync("data/dbip-asn-ipv4.mmdb");
const lookup2 = new Reader<AsnResponse>(buffer2);

export function GetIPInfo(ip: string): { country: string, latitude: number, longitude: number } {
    const result = lookup.get(ip);

    if (!result) return { country: "Unknown", latitude: 0, longitude: 0 };

    // @ts-expect-error
    return { country: result.country_code.toLowerCase() || "un", latitude: result.latitude || 0, longitude: result.longitude || 0 };
}

export function GetIPASN(ip: string): { asn: number } {
    const result = lookup2.get(ip)
    if (!result) return { asn: 0 }

    return { asn: result.autonomous_system_number };
}