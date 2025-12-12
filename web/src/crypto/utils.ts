export function generateSalt(length: number = 16): string {
  const array = new Uint8Array(length);
  crypto.getRandomValues(array);

  return arrayBufferToBase64(array.buffer);
}

export function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }

  return btoa(binary);
}

export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }

  return bytes.buffer;
}

export function calculateChunks(fileSize: number, chunkSize: number): number {
  return Math.ceil(fileSize / chunkSize);
}

export async function getFileChunk(
  file: File,
  index: number,
  chunkSize: number,
): Promise<Blob> {
  const start = index * chunkSize;
  const end = Math.min(start + chunkSize, file.size);
  return file.slice(start, end);
}

export async function calculateChunkHash(chunk: Blob): Promise<string> {
    const buffer = await chunk.arrayBuffer();
    const hashedData = await crypto.subtle.digest("SHA-256", buffer);

    const uint8Array = new Uint8Array(hashedData);
    if (uint8Array.toHex) {
        return uint8Array.toHex();
    }

    return Array.from(uint8Array)
        .map(b => b.toString(16).padStart(2, '0'))
        .join('');
}
