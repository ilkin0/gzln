export interface InitUploadRequest {
  salt: string;
  encrypted_filename: string;
  encrypted_mime_type: string;
  total_size: number;
  chunk_count: number;
  chunk_size: number;
  expires_in_hours?: number;
  max_downloads?: number;
  pbkdf2_iterations: number;
}

export interface InitUploadResponse {
  file_id: string;
  share_id: string;
  upload_token: string;
  expires_at: string;
}

export interface FileMetadata {
  share_id: string;
  encrypted_filename: string;
  encrypted_mime_type: string;
  salt: string;
  total_size: number;
  chunk_count: number;
  expires_at: string;
  max_downloads: number;
  download_count: number;
}

export interface ApiError {
  message: string;
  status: number;
}
