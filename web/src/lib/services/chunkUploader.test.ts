import { describe, it, expect, vi, beforeEach } from "vitest";
import { uploadFileInChunks } from "./chunkUploader";
import { deriveKey } from "../../crypto/encrypt";
import { generateSalt } from "../../crypto/utils";
import * as filesApi from "$lib/api/files";

vi.mock("$lib/api/files", () => ({
  filesApi: {
    uploadChunk: vi.fn(),
  },
}));

describe("ChunkUploader Service", () => {
  const TEST_PASSWORD = "test-password";

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("uploadFileInChunks", () => {
    it("should upload file in correct number of chunks", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      // Create a 25-byte file (will create 3 chunks of 10 bytes)
      const data = new Uint8Array(25);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
      });

      expect(mockUploadChunk).toHaveBeenCalledTimes(3);
    });

    it("should upload chunks with correct indices", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(25);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
      });

      expect(mockUploadChunk).toHaveBeenNthCalledWith(
        1,
        "test-file-id",
        0,
        expect.any(Blob),
        expect.any(String),
        "test-token",
      );

      expect(mockUploadChunk).toHaveBeenNthCalledWith(
        2,
        "test-file-id",
        1,
        expect.any(Blob),
        expect.any(String),
        "test-token",
      );

      expect(mockUploadChunk).toHaveBeenNthCalledWith(
        3,
        "test-file-id",
        2,
        expect.any(Blob),
        expect.any(String),
        "test-token",
      );
    });

    it("should upload encrypted chunks (larger than plain chunks)", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(20);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
      });

      const firstCallArgs = mockUploadChunk.mock.calls[0];
      const uploadedChunk = firstCallArgs[2] as Blob;

      expect(uploadedChunk.size).toBe(38);
    });

    it("should call onProgress callback with progress updates", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(30);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      const onProgress = vi.fn();

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
        onProgress,
      });

      // Should be called multiple times (initial + 3 chunks)
      expect(onProgress.mock.calls.length).toBeGreaterThan(3);

      const lastProgress =
        onProgress.mock.calls[onProgress.mock.calls.length - 1][0];
      expect(lastProgress.uploadedChunks).toBe(3);
      expect(lastProgress.totalChunks).toBe(3);
      expect(lastProgress.uploadedBytes).toBe(30);
      expect(lastProgress.totalBytes).toBe(30);
    });

    it("should handle single chunk file", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(5);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
      });

      expect(mockUploadChunk).toHaveBeenCalledTimes(1);
    });

    it("should calculate speed and estimated time in progress", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(30);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({receivedHash: "", chunkIndex: 0, status: "success" });

      const onProgress = vi.fn();

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
        onProgress,
      });

      const lastProgress =
        onProgress.mock.calls[onProgress.mock.calls.length - 1][0];

      expect(lastProgress.currentSpeed).toBeGreaterThan(0);
      expect(typeof lastProgress.estimatedTimeRemaining).toBe("number");
    });

    it("should respect concurrency limit", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(100);
      const file = new File([data], "test.bin");

      let concurrentCalls = 0;
      let maxConcurrentCalls = 0;

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockImplementation(() => {
        concurrentCalls++;
        maxConcurrentCalls = Math.max(maxConcurrentCalls, concurrentCalls);

        return new Promise((resolve) => {
          setTimeout(() => {
            concurrentCalls--;
            resolve({ status: "success", chunkIndex: 0, receivedHash: "", });
          }, 10);
        });
      });

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
        concurrency: 3,
      });

      expect(maxConcurrentCalls).toBeLessThanOrEqual(3);
    });

    it("should call onProgress immediately with initial state", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(30);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockResolvedValue({ chunkIndex: 0, status: "success", receivedHash: "" });

      const onProgress = vi.fn();

      await uploadFileInChunks({
        file,
        fileId: "test-file-id",
        uploadToken: "test-token",
        chunkSize: 10,
        encryptionKey: key,
        onProgress,
      });

      const firstCall = onProgress.mock.calls[0][0];
      expect(firstCall.uploadedChunks).toBe(0);
      expect(firstCall.totalChunks).toBe(3);
      expect(firstCall.uploadedBytes).toBe(0);
      expect(firstCall.totalBytes).toBe(30);
      expect(firstCall.currentSpeed).toBe(0);
      expect(firstCall.estimatedTimeRemaining).toBe(0);
    });

    it("should fail fast when first chunk upload fails", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(100); // 10 chunks
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockImplementation(() =>
        Promise.reject(new Error("Network error")),
      );

      await expect(
        uploadFileInChunks({
          file,
          fileId: "test-file-id",
          uploadToken: "test-token",
          chunkSize: 10,
          encryptionKey: key,
        }),
      ).rejects.toThrow("Network error");

      expect(mockUploadChunk.mock.calls.length).toBeLessThan(10);
    });

    it("should call onError callback when chunk upload fails", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(30);
      const file = new File([data], "test.bin");

      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      const uploadError = new Error("Chunk upload failed");
      mockUploadChunk.mockImplementation(() => Promise.reject(uploadError));

      const onError = vi.fn();

      await expect(
        uploadFileInChunks({
          file,
          fileId: "test-file-id",
          uploadToken: "test-token",
          chunkSize: 10,
          encryptionKey: key,
          onError,
        }),
      ).rejects.toThrow("Chunk upload failed");

      expect(onError).toHaveBeenCalledWith(uploadError, expect.any(Number));
    });

    it("should not start new chunks after first error", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const data = new Uint8Array(100); // 10 chunks with concurrency 2
      const file = new File([data], "test.bin");

      let callCount = 0;
      const mockUploadChunk = vi.mocked(filesApi.filesApi.uploadChunk);
      mockUploadChunk.mockImplementation(() => {
        callCount++;
        return Promise.reject(new Error("Server error"));
      });

      await expect(
        uploadFileInChunks({
          file,
          fileId: "test-file-id",
          uploadToken: "test-token",
          chunkSize: 10,
          encryptionKey: key,
          concurrency: 2,
        }),
      ).rejects.toThrow("Server error");

      expect(callCount).toBeLessThan(10);
    });
  });
});
