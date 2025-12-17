import {filesApi} from "$lib/api/files";
import {decryptChunk} from "../../crypto/encrypt";
import {responseToBlob} from "$lib/utils/stream";

export interface DownloadProgress {
    chunkIndex: number;
    bytesReceived: number;
    totalBytes: number;
}

export interface ChunkDownloadOptions {
    shareId: string;
    totalChunks: number;
    decryptionKey: CryptoKey;
    onProgress?: (progress: DownloadProgress) => void;
}

export async function downloadFileInChunks(
    options: ChunkDownloadOptions
): Promise<Uint8Array[]> {
    const {shareId, totalChunks, decryptionKey, onProgress} = options;
    const chunks: Uint8Array[] = [];

    for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
        const response = await filesApi.downloadChunk(shareId, chunkIndex);
        const blob = await responseToBlob(response, (streamProgress) => {
            if (onProgress) {
                onProgress({
                    chunkIndex,
                    bytesReceived: streamProgress.bytesReceived,
                    totalBytes: streamProgress.totalBytes,
                });
            }
        });

        const decryptedChunk = await decryptChunk(blob, decryptionKey);
        const blobBuffer = await decryptedChunk.arrayBuffer();
        chunks.push(new Uint8Array(blobBuffer));
    }

    return chunks;
}
