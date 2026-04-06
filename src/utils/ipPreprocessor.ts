export function AddressToBuffer(addr: string): Buffer | null {
    const parts = addr.split(":");

    if (parts.length !== 2) {
        return null;
    }

    const [ip, portStr] = parts;

    const octets = ip.split(".");
    if (octets.length !== 4) {
        return null;
    }

    const buf = Buffer.allocUnsafe(6);

    for (let i = 0; i < 4; i++) {
        const octet = Number(octets[i]);

        if (!Number.isInteger(octet) || octet < 0 || octet > 255) {
            return null;
        }

        buf[i] = octet;
    }

    const port = Number(portStr);

    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        return null;
    }

    buf.writeUInt16BE(port, 4);

    return buf;
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