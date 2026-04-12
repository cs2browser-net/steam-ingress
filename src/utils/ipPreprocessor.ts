const ADDRESS_BUFFER_SIZE = 6;
const ADDRESS_BUFFER_POOL_LIMIT = 4096;
const addressBufferPool: Buffer[] = [];

export type ReusableAddressBuffer = {
    buffer: Buffer;
    release: () => void;
};

function acquireAddressBuffer(): Buffer {
    const pooled = addressBufferPool.pop();
    if (pooled) return pooled;

    return Buffer.allocUnsafe(ADDRESS_BUFFER_SIZE);
}

function releaseAddressBuffer(buffer: Buffer): void {
    if (buffer.length !== ADDRESS_BUFFER_SIZE) return;
    if (addressBufferPool.length >= ADDRESS_BUFFER_POOL_LIMIT) return;

    addressBufferPool.push(buffer);
}

function createReusableAddressBuffer(buffer: Buffer): ReusableAddressBuffer {
    let released = false;

    return {
        buffer,
        release: () => {
            if (released) return;
            released = true;
            releaseAddressBuffer(buffer);
        },
    };
}

export function AddressToBuffer(addr: string): ReusableAddressBuffer | null {
    const parts = addr.split(":");

    if (parts.length !== 2) {
        return null;
    }

    const [ip, portStr] = parts;

    const octets = ip.split(".");
    if (octets.length !== 4) {
        return null;
    }

    const buf = acquireAddressBuffer();

    for (let i = 0; i < 4; i++) {
        const octet = Number(octets[i]);

        if (!Number.isInteger(octet) || octet < 0 || octet > 255) {
            releaseAddressBuffer(buf);
            return null;
        }

        buf[i] = octet;
    }

    const port = Number(portStr);

    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        releaseAddressBuffer(buf);
        return null;
    }

    buf.writeUInt16BE(port, 4);

    return createReusableAddressBuffer(buf);
}

export function BufferToAddress(buf: Buffer): string | null {
    if (!Buffer.isBuffer(buf)) {
        return null;
    }

    if (buf.length !== 6) {
        return null;
    }

    const ip = `${buf[0]}.${buf[1]}.${buf[2]}.${buf[3]}`;
    const port = buf.readUInt16BE(4);

    return `${ip}:${port}`;
}