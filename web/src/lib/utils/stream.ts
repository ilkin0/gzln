/**
 * Utility functions for handling streaming data and blob conversion
 */

export interface StreamProgress {
  bytesReceived: number;
  totalBytes: number;
  percentage: number;
}

/**
 * Convert a Response to a Blob with optional progress tracking
 *
 * @param response - Fetch Response object
 * @param onProgress - Optional progress callback
 * @returns Promise<Blob>
 */
export async function responseToBlob(
  response: Response,
  onProgress?: (progress: StreamProgress) => void
): Promise<Blob> {
  // If no progress tracking needed, return blob directly
  if (!onProgress || !response.body) {
    return await response.blob();
  }

  const contentLength = response.headers.get("Content-Length");
  const totalBytes = contentLength ? parseInt(contentLength, 10) : 0;

  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let bytesReceived = 0;

  while (true) {
    const { done, value } = await reader.read();

    if (done) break;

    chunks.push(value);
    bytesReceived += value.length;

    const percentage = totalBytes > 0 ? (bytesReceived / totalBytes) * 100 : 0;

    onProgress({
      bytesReceived,
      totalBytes,
      percentage,
    });
  }

  return new Blob(chunks as BlobPart[]);
}

/**
 * Convert a Blob to ArrayBuffer
 *
 * @param blob - Blob to convert
 * @returns Promise<ArrayBuffer>
 */
export async function blobToArrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  return await blob.arrayBuffer();
}

/**
 * Convert a Blob to Uint8Array
 *
 * @param blob - Blob to convert
 * @returns Promise<Uint8Array>
 */
export async function blobToUint8Array(blob: Blob): Promise<Uint8Array> {
  const buffer = await blob.arrayBuffer();
  return new Uint8Array(buffer);
}


/**
 * Read a stream with progress tracking and return as Uint8Array
 *
 * @param reader - ReadableStreamDefaultReader
 * @param totalBytes - Total expected bytes (for progress calculation)
 * @param onProgress - Progress callback
 * @returns Promise<Uint8Array>
 */
export async function readStreamWithProgress(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  totalBytes: number,
  onProgress?: (progress: StreamProgress) => void
): Promise<Uint8Array> {
  const chunks: Uint8Array[] = [];
  let bytesReceived = 0;

  while (true) {
    const { done, value } = await reader.read();

    if (done) break;

    chunks.push(value);
    bytesReceived += value.length;

    if (onProgress) {
      const percentage = totalBytes > 0 ? (bytesReceived / totalBytes) * 100 : 0;
      onProgress({
        bytesReceived,
        totalBytes,
        percentage,
      });
    }
  }

  // Combine all chunks into a single Uint8Array
  const totalLength = chunks.reduce((acc, chunk) => acc + chunk.length, 0);
  const result = new Uint8Array(totalLength);
  let offset = 0;

  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }

  return result;
}
