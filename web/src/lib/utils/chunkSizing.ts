export function getOptimalChunkSize(fileSize: number): number {
  const MB = 1024 * 1024;

  // Small files: 5MB chunks
  if (fileSize < 100 * MB) {
    return 5 * MB;
  }

  // Medium files (100MB - 500MB): 10MB chunks
  if (fileSize < 500 * MB) {
    return 10 * MB;
  }

  // Large files (500MB - 2GB): 25MB chunks
  if (fileSize < 2 * 1024 * MB) {
    return 25 * MB;
  }

  // Very large files (2GB+): 50MB chunks
  return 50 * MB;
}

export function estimateRequestCount(fileSize: number): number {
  const chunkSize = getOptimalChunkSize(fileSize);
  return Math.ceil(fileSize / chunkSize);
}

export function getChunkSizeInfo(fileSize: number) {
  const chunkSize = getOptimalChunkSize(fileSize);
  const requestCount = estimateRequestCount(fileSize);
  const chunkSizeMB = (chunkSize / (1024 * 1024)).toFixed(0);

  return {
    chunkSize,
    chunkSizeMB,
    requestCount,
    description: `${chunkSizeMB}MB chunks, ~${requestCount} requests`
  };
}
