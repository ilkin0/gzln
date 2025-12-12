// Type extension for Uint8Array.toHex() method. It is not supported by TS definition yet
declare global {
    interface Uint8Array {
        toHex?(): string;
    }
}

export {};