export function AddressToBuffer(addr: string): Buffer {
    const parts = addr.split(":");

    if (parts.length !== 2) {
        throw new Error("Invalid format. Expected IPv4:Port");
    }

    const [ip, portStr] = parts;

    const octets = ip.split(".");
    if (octets.length !== 4) {
        throw new Error("Invalid IPv4 address");
    }

    const buf = Buffer.allocUnsafe(6);

    for (let i = 0; i < 4; i++) {
        const octet = Number(octets[i]);

        if (!Number.isInteger(octet) || octet < 0 || octet > 255) {
            throw new Error(`Invalid IPv4 octet: ${octets[i]}`);
        }

        buf[i] = octet;
    }

    const port = Number(portStr);

    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        throw new Error(`Invalid port: ${portStr}`);
    }

    buf.writeUInt16BE(port, 4);

    return buf;
}

export function BufferToAdddress(buf: Buffer): string {
    if (!Buffer.isBuffer(buf)) {
        throw new Error("Input must be a Buffer");
    }

    if (buf.length !== 6) {
        throw new Error("Buffer must be exactly 6 bytes");
    }

    const ip = `${buf[0]}.${buf[1]}.${buf[2]}.${buf[3]}`;
    const port = buf.readUInt16BE(4);

    return `${ip}:${port}`;
}