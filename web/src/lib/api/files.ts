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

  // TODO: chunk upload
  // async uploadChunk(shareId: string, chunkIndex: number, data: Blob, token: string) { ... }
};
