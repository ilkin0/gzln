import { apiClient } from "./client";
import type {
  InitUploadRequest,
  InitUploadResponse,
  FileMetadata,
} from "$lib/types/api";

export const filesApi = {
  async initUpload(data: InitUploadRequest): Promise<InitUploadResponse> {
    return apiClient.post<InitUploadResponse>("/api/files/upload/init", data);
  },

  async getFileMetadata(shareId: string): Promise<FileMetadata> {
    return apiClient.get<FileMetadata>(`/api/files/${shareId}`);
  },

  async uploadChunk(
    fileId: string,
    chunkIndex: number,
    chunk: Blob,
    hash: string,
    uploadToken: string,
  ): Promise<void> {
    const formData = new FormData();
    formData.append("chunk", chunk);
    formData.append("chunk_index", chunkIndex.toString());
    formData.append("hash", hash);

    try {
      const response = await fetch(`/api/files/${fileId}/chunks`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${uploadToken}`,
        },
        body: formData,
      });

      if (!response.ok) {
        throw new Error(
          `Chunk upload failed: ${response.status} ${response.statusText}`,
        );
      }
    } catch (error) {
      if (error instanceof TypeError) {
        throw new Error("Network error - please check your connection");
      }
      throw error;
    }
  },
};
