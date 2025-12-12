import {apiClient} from "./client";
import type {
  ApiResponse,
  ChunkUploadResponse,
  FileMetadata,
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
    
    try {
      const response = await fetch(`/api/v1/files/${fileId}/chunks`, {
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

      const json: ApiResponse<ChunkUploadResponse> = await response.json();

      if (!json.success) {
        throw new Error(json.message || "Upload failed");
      }

      return json.data!;
    } catch (error) {
      if (error instanceof TypeError) {
        throw new Error("Network error - please check your connection");
      }
      throw error;
    }
  },
};
