// App configuration
export const API_BASE_URL =
  import.meta.env.VITE_API_URL || "http://localhost:8080";

export const MAX_FILE_SIZE = 512 * 1024 * 1024; // 512 MB
export const CHUNK_SIZE = 5 * 1024 * 1024; // 5 MB
export const DEFAULT_EXPIRES_HOURS = 24;
export const DEFAULT_MAX_DOWNLOADS = 3;
