export function log(message: any) {
    const now = new Date().toISOString();
    console.log(`[${now}] ${message}`);
}