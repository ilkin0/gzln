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
  }
};
