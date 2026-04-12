import { Socket } from "node:net";

enum QueryByte {
    ASN = 0,
    City = 1,
    Country = 2,
}

enum StatusByte {
    Ok = 0,
    BadRequest = 1,
    NotFound = 2,
    Unavailable = 3,
    InternalError = 4,
}

type QueryResponse = {
    status: number;
    payload: Buffer;
};

const GEOIP_HOST = process.env.GEOIP_HOST ?? "127.0.0.1";
const parsedGeoIPPort = Number(process.env.GEOIP_PORT ?? "8081");
const GEOIP_PORT = Number.isInteger(parsedGeoIPPort) && parsedGeoIPPort > 0 && parsedGeoIPPort <= 65535
    ? parsedGeoIPPort
    : 8081;
const GEOIP_POOL_SIZE = Number(process.env.GEOIP_POOL_SIZE ?? "12");
const GEOIP_REQUEST_TIMEOUT_MS = Number(process.env.GEOIP_REQUEST_TIMEOUT_MS ?? "2500");
const MAX_FRAME_SIZE = 16 * 1024 * 1024;
const utf8Decoder = new TextDecoder("utf-8", { fatal: true });

class PayloadReader {
    private offset = 0;

    constructor(private readonly payload: Buffer) { }

    private readBytes(length: number): Buffer {
        if (this.offset + length > this.payload.length) {
            throw new Error("Unexpected end of payload");
        }

        const bytes = this.payload.subarray(this.offset, this.offset + length);
        this.offset += length;
        return bytes;
    }

    readUInt8(): number {
        const bytes = this.readBytes(1);
        return bytes.readUInt8(0);
    }

    readUInt16BE(): number {
        const bytes = this.readBytes(2);
        return bytes.readUInt16BE(0);
    }

    readUInt32BE(): number {
        const bytes = this.readBytes(4);
        return bytes.readUInt32BE(0);
    }

    readFloat64BE(): number {
        const bytes = this.readBytes(8);
        return bytes.readDoubleBE(0);
    }

    readOptionalString(): string | null {
        const present = this.readUInt8();
        if (present === 0) return null;
        if (present !== 1) throw new Error(`Invalid optional string presence flag: ${present}`);

        const length = this.readUInt16BE();
        const chars = this.readBytes(length);

        return utf8Decoder.decode(chars);
    }

    readOptionalUInt32(): number | null {
        const present = this.readUInt8();
        if (present === 0) return null;
        if (present !== 1) throw new Error(`Invalid optional u32 presence flag: ${present}`);

        return this.readUInt32BE();
    }

    readOptionalFloat64(): number | null {
        const present = this.readUInt8();
        if (present === 0) return null;
        if (present !== 1) throw new Error(`Invalid optional f64 presence flag: ${present}`);

        return this.readFloat64BE();
    }

    ensureConsumed(): void {
        if (this.offset !== this.payload.length) {
            throw new Error(`Unexpected trailing bytes: ${this.payload.length - this.offset}`);
        }
    }
}

class GeoIPConnection {
    private socket: Socket | null = null;
    private connecting: Promise<void> | null = null;
    private readBuffer = Buffer.alloc(0);
    private pending: {
        resolve: (value: QueryResponse) => void;
        reject: (reason: Error) => void;
        timeoutHandle: NodeJS.Timeout;
    } | null = null;

    constructor(
        private readonly host: string,
        private readonly port: number,
        private readonly requestTimeoutMs: number,
    ) { }

    async request(frame: Buffer): Promise<QueryResponse> {
        await this.ensureConnected();

        if (!this.socket || this.socket.destroyed) {
            throw new Error("GeoIP socket is not connected");
        }

        if (this.pending) {
            throw new Error("GeoIP connection is busy");
        }

        return await new Promise<QueryResponse>((resolve, reject) => {
            const timeoutHandle = setTimeout(() => {
                this.failPending(new Error("GeoIP request timed out"));
                this.resetSocket();
            }, this.requestTimeoutMs);

            this.pending = {
                resolve,
                reject: (error: Error) => reject(error),
                timeoutHandle,
            };

            this.socket!.write(frame, (error?: Error | null) => {
                if (error) {
                    this.failPending(error);
                    this.resetSocket();
                }
            });
        });
    }

    async close(): Promise<void> {
        this.failPending(new Error("GeoIP connection was closed"));
        this.resetSocket();
    }

    private async ensureConnected(): Promise<void> {
        if (this.socket && !this.socket.destroyed) {
            return;
        }

        if (this.connecting) {
            await this.connecting;
            return;
        }

        this.connecting = new Promise<void>((resolve, reject) => {
            const socket = new Socket();
            socket.setNoDelay(true);

            const onConnect = () => {
                socket.removeListener("error", onConnectError);
                this.socket = socket;
                this.readBuffer = Buffer.alloc(0);

                socket.on("data", (chunk: Buffer) => {
                    this.readBuffer = Buffer.concat([this.readBuffer, chunk]);
                    this.tryResolvePending();
                });

                socket.on("error", () => {
                    this.failPending(new Error("GeoIP socket error"));
                    this.resetSocket(false);
                });

                socket.on("close", () => {
                    this.failPending(new Error("GeoIP socket closed"));
                    this.resetSocket(false);
                });

                resolve();
            };

            const onConnectError = (error: Error) => {
                socket.removeListener("connect", onConnect);
                socket.destroy();
                this.socket = null;
                reject(error);
            };

            socket.once("connect", onConnect);
            socket.once("error", onConnectError);
            socket.connect(this.port, this.host);
        });

        try {
            await this.connecting;
        } finally {
            this.connecting = null;
        }
    }

    private tryResolvePending(): void {
        if (!this.pending) return;
        if (this.readBuffer.length < 5) return;

        const status = this.readBuffer.readUInt8(0);
        const payloadLength = this.readBuffer.readUInt32BE(1);

        if (payloadLength > MAX_FRAME_SIZE) {
            this.failPending(new Error(`GeoIP payload too large: ${payloadLength} bytes`));
            this.resetSocket();
            return;
        }

        const frameLength = 5 + payloadLength;

        if (this.readBuffer.length < frameLength) return;

        const payload = this.readBuffer.subarray(5, frameLength);
        this.readBuffer = this.readBuffer.subarray(frameLength);

        const pending = this.pending;
        this.pending = null;
        clearTimeout(pending.timeoutHandle);
        pending.resolve({ status, payload });
    }

    private failPending(error: Error): void {
        if (!this.pending) return;

        const pending = this.pending;
        this.pending = null;
        clearTimeout(pending.timeoutHandle);
        pending.reject(error);
    }

    private resetSocket(destroy = true): void {
        if (!this.socket) return;

        const socket = this.socket;
        this.socket = null;
        this.readBuffer = Buffer.alloc(0);

        if (destroy && !socket.destroyed) {
            socket.destroy();
        }
    }
}

class GeoIPConnectionPool {
    private readonly connections: GeoIPConnection[];
    private readonly available: GeoIPConnection[];
    private readonly waiters: Array<(connection: GeoIPConnection) => void> = [];

    constructor(
        host: string,
        port: number,
        poolSize: number,
        requestTimeoutMs: number,
    ) {
        this.connections = Array.from(
            { length: Math.max(1, poolSize) },
            () => new GeoIPConnection(host, port, requestTimeoutMs),
        );
        this.available = [...this.connections];
    }

    async query(query: QueryByte, ip: string): Promise<QueryResponse> {
        const connection = await this.acquire();

        try {
            return await connection.request(this.buildSingleQueryFrame(query, ip));
        } finally {
            this.release(connection);
        }
    }

    async close(): Promise<void> {
        await Promise.all(this.connections.map(async (connection) => await connection.close()));
    }

    private async acquire(): Promise<GeoIPConnection> {
        if (this.available.length > 0) {
            return this.available.pop()!;
        }

        return await new Promise<GeoIPConnection>((resolve) => {
            this.waiters.push(resolve);
        });
    }

    private release(connection: GeoIPConnection): void {
        const waiter = this.waiters.shift();
        if (waiter) {
            waiter(connection);
            return;
        }

        this.available.push(connection);
    }

    private buildSingleQueryFrame(query: QueryByte, ip: string): Buffer {
        const ipBytes = Buffer.from(ip, "utf8");
        if (ipBytes.length === 0 || ipBytes.length > 255) {
            throw new Error("GeoIP query IP length must be between 1 and 255 bytes");
        }

        const frame = Buffer.allocUnsafe(2 + ipBytes.length);
        frame.writeUInt8(query, 0);
        frame.writeUInt8(ipBytes.length, 1);
        ipBytes.copy(frame, 2);

        return frame;
    }
}

const geoIPPool = new GeoIPConnectionPool(
    GEOIP_HOST,
    GEOIP_PORT,
    GEOIP_POOL_SIZE,
    GEOIP_REQUEST_TIMEOUT_MS,
);

process.once("beforeExit", () => {
    void geoIPPool.close();
});

function normalizeIPInput(input: string): string {
    if (input.startsWith("[")) {
        const endIndex = input.indexOf("]");
        if (endIndex > 1) {
            return input.slice(1, endIndex);
        }
    }

    const firstColon = input.indexOf(":");
    const lastColon = input.lastIndexOf(":");
    if (firstColon !== -1 && firstColon === lastColon) {
        return input.slice(0, lastColon);
    }

    return input;
}

function parseCityPayload(payload: Buffer): { country: string; latitude: number; longitude: number } {
    const reader = new PayloadReader(payload);

    const countryCode = reader.readOptionalString();
    reader.readOptionalString();
    reader.readOptionalString();
    reader.readOptionalString();
    reader.readOptionalString();

    const latitude = reader.readOptionalFloat64();
    const longitude = reader.readOptionalFloat64();
    reader.readOptionalString();

    reader.ensureConsumed();

    return {
        country: countryCode ? countryCode.toLowerCase() : "Unknown",
        latitude: typeof latitude === "number" && Number.isFinite(latitude) ? latitude : 0,
        longitude: typeof longitude === "number" && Number.isFinite(longitude) ? longitude : 0,
    };
}

function parseASNPayload(payload: Buffer): { asn: number } {
    const reader = new PayloadReader(payload);
    const asn = reader.readOptionalUInt32();
    reader.readOptionalString();

    reader.ensureConsumed();

    return { asn: typeof asn === "number" && Number.isInteger(asn) ? asn : 0 };
}

export async function GetIPInfo(ip: string): Promise<{ country: string, latitude: number, longitude: number }> {
    try {
        const response = await geoIPPool.query(QueryByte.City, normalizeIPInput(ip));
        if (response.status !== StatusByte.Ok) {
            return { country: "Unknown", latitude: 0, longitude: 0 };
        }

        return parseCityPayload(response.payload);
    } catch {
        return { country: "Unknown", latitude: 0, longitude: 0 };
    }
}

export async function GetIPASN(ip: string): Promise<{ asn: number }> {
    try {
        const response = await geoIPPool.query(QueryByte.ASN, normalizeIPInput(ip));
        if (response.status !== StatusByte.Ok) {
            return { asn: 0 };
        }

        return parseASNPayload(response.payload);
    } catch {
        return { asn: 0 };
    }
}