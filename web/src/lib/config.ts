// App configuration
export const API_BASE_URL =
  import.meta.env.VITE_API_URL || "http://localhost:8080";

export const MAX_FILE_SIZE = 5 * 1024 * 1024 * 1024; // 5 GB
export const CHUNK_SIZE = 5 * 1024 * 1024; // 5 MB
export const DEFAULT_EXPIRES_HOURS = 72;
export const DEFAULT_MAX_DOWNLOADS = 100;
