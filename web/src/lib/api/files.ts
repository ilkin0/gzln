import {apiClient} from "./client";
import type {
  ChunkUploadResponse,
  FileMetadata,
  FinalizeUploadResponse,
  InitUploadRequest,
  InitUploadResponse
} from "$lib/types/api";

export const filesApi = {
  async initUpload(data: InitUploadRequest): Promise<InitUploadResponse> {
    return apiClient.post<InitUploadResponse>("/api/v1/files/upload/init", data);
  },

  async getFileMetadata(shareId: string): Promise<FileMetadata> {
    return apiClient.get<FileMetadata>(`/api/v1/download/${shareId}/metadata`);
  },

  async uploadChunk(
    fileId: string,
    chunkIndex: number,
    chunk: Blob,
    hash: string,
    uploadToken: string,
  ): Promise<ChunkUploadResponse> {
    const formData = new FormData();
    formData.append("chunk", chunk);
    formData.append("chunk_index", chunkIndex.toString());
    formData.append("hash", hash);

    return apiClient.postForm<ChunkUploadResponse>(
        `/api/v1/files/${fileId}/chunks`,
        formData,
        {Authorization: `Bearer ${uploadToken}`}
    );
  },

  async finalizeUpload(fileId: string): Promise<FinalizeUploadResponse> {
    return apiClient.post<FinalizeUploadResponse>(`/api/v1/files/${fileId}/finalize`);
  },

  async getMockFileMetadata(shareId: string): Promise<FileMetadata> {
    await new Promise(resolve => setTimeout(resolve, 800));

    if (shareId === 'expired' || shareId === 'not-found') {
      throw new Error('404: File not found or expired');
    }
    if (shareId === 'error') {
      throw new Error('500: Server error');
    }

    const mdata = await this.getFileMetadata(shareId)
    const expiresAtFormatted = new Date(mdata.expires_at).toISOString();

    return {
      encrypted_filename: mdata.encrypted_filename,
      encrypted_mime_type: mdata.encrypted_mime_type,
      salt: mdata.salt,
      total_size: mdata.total_size,
      chunk_count: mdata.chunk_count,
      expires_at: expiresAtFormatted,
      max_downloads: mdata.max_downloads,
      download_count: mdata.download_count,
    };
  },
};
