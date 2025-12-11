import { filesApi } from "$lib/api/files";
import {
  calculateChunks,
  getFileChunk,
  calculateChunkHash,
} from "../../crypto/utils";
import { encryptChunk } from "../../crypto/encrypt";

const CONCURRENT_UPLOADS = 5;

export interface UploadProgress {
  uploadedChunks: number;
  totalChunks: number;
  uploadedBytes: number;
  totalBytes: number;
  currentSpeed: number;
  estimatedTimeRemaining: number;
}

export interface ChunkUploadOptions {
  file: File;
  fileId: string;
  uploadToken: string;
  chunkSize: number;
  encryptionKey: CryptoKey;
  onProgress?: (progress: UploadProgress) => void;
  onError?: (error: Error, chunkIndex: number) => void;
  concurrency?: number;
}

export async function uploadFileInChunks(
  options: ChunkUploadOptions,
): Promise<void> {
  const {
    file,
    fileId,
    uploadToken,
    chunkSize,
    encryptionKey,
    onProgress,
    onError,
    concurrency = CONCURRENT_UPLOADS,
  } = options;

  const totalChunks = calculateChunks(file.size, chunkSize);
  let uploadedChunks = 0;
  let uploadedBytes = 0;
  const startTime = Date.now();

  if (onProgress) {
    onProgress({
      uploadedChunks: 0,
      totalChunks,
      uploadedBytes: 0,
      totalBytes: file.size,
      currentSpeed: 0,
      estimatedTimeRemaining: 0,
    });
  }

  const chunkIndices = Array.from({ length: totalChunks }, (_, i) => i);
  const activeUploads = new Set<Promise<void>>();
  let uploadAborted = false;

  async function uploadChunk(chunkIndex: number): Promise<void> {
    if (uploadAborted) {
      throw new Error("Upload aborted due to previous error");
    }
    try {
      const chunk = await getFileChunk(file, chunkIndex, chunkSize);
      const plainChunkSize = chunk.size;

      const encryptedChunk = await encryptChunk(chunk, encryptionKey);
      const hash = await calculateChunkHash(encryptedChunk);
      
      const response: ApiResponse<ChunkUploadResponse> = await filesApi.uploadChunk(
        fileId,
        chunkIndex,
        encryptedChunk,
        hash,
        uploadToken,
      );

      uploadedChunks++;
      uploadedBytes += plainChunkSize;

      if (onProgress) {
        const elapsedSeconds = (Date.now() - startTime) / 1000;
        const currentSpeed = uploadedBytes / elapsedSeconds;
        const remainingBytes = file.size - uploadedBytes;
        const estimatedTimeRemaining = remainingBytes / currentSpeed;

        onProgress({
          uploadedChunks,
          totalChunks,
          uploadedBytes,
          totalBytes: file.size,
          currentSpeed,
          estimatedTimeRemaining,
        });
      }
    } catch (error) {
      uploadAborted = true;
      if (onError) {
        onError(
          error instanceof Error ? error : new Error("Upload Failed"),
          chunkIndex,
        );
      }
      throw error;
    }
  }

  try {
    while (chunkIndices.length > 0 || activeUploads.size > 0) {
      while (chunkIndices.length > 0 && activeUploads.size < concurrency) {
        const chunkIndex = chunkIndices.shift()!;
        const uploadPromise = uploadChunk(chunkIndex);

        activeUploads.add(uploadPromise);

        uploadPromise
          .catch(() => {
            // Errors are already handled in uploadChunk's catch block
            // This just prevents unhandled rejection warnings
          })
          .finally(() => {
            activeUploads.delete(uploadPromise);
          });
      }

      if (activeUploads.size > 0) {
        await Promise.race(activeUploads);
      }
    }
  } catch (error) {
    chunkIndices.length = 0;
    uploadAborted = true;

    throw error;
  }
}
