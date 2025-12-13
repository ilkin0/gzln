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
    return apiClient.get<FileMetadata>(`/api/v1/files/${shareId}`);
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

    const twoDaysFromNow = new Date();
    twoDaysFromNow.setDate(twoDaysFromNow.getDate() + 2);

    // Backend for Download feature is WIP. Generate real encrypted mock data
    const { deriveKey, encryptString, generateSalt } = await import('../../crypto/encrypt');

    const mockPassword = 'mockpassword123';
    const salt = generateSalt();
    const key = await deriveKey(mockPassword, salt);

    const encryptedFilename = await encryptString('example-document.pdf', key);
    const encryptedMimeType = await encryptString('application/pdf', key);

    return {
      share_id: shareId,
      encrypted_filename: encryptedFilename,
      encrypted_mime_type: encryptedMimeType,
      salt: salt,
      total_size: 15728640,
      chunk_count: 3,
      expires_at: twoDaysFromNow.toISOString(),
      max_downloads: 10,
      download_count: 3
    };
  }
};
