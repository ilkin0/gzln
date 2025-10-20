import { describe, it, expect } from "vitest";
import {
  generateSalt,
  arrayBufferToBase64,
  base64ToArrayBuffer,
  calculateChunks,
  getFileChunk,
  calculateChunkHash,
} from "./utils";

describe("Crypto Utilities", () => {
  describe("generateSalt", () => {
    it("should generate a base64 salt of default length", () => {
      const salt = generateSalt();

      expect(typeof salt).toBe("string");
      expect(salt.length).toBeGreaterThan(0);
    });

    it("should generate different salts each time", () => {
      const salt1 = generateSalt();
      const salt2 = generateSalt();

      expect(salt1).not.toBe(salt2);
    });

    it("should generate salt of custom length", () => {
      const salt = generateSalt(32);
      const decoded = base64ToArrayBuffer(salt);

      expect(decoded.byteLength).toBe(32);
    });
  });

  describe("Base64 Encoding/Decoding", () => {
    it("should encode and decode ArrayBuffer correctly", () => {
      const original = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
      const buffer = original.buffer;

      const base64 = arrayBufferToBase64(buffer);
      const decoded = base64ToArrayBuffer(base64);
      const result = new Uint8Array(decoded);

      expect(result).toEqual(original);
    });

    it("should handle empty buffer", () => {
      const original = new Uint8Array([]);
      const buffer = original.buffer;

      const base64 = arrayBufferToBase64(buffer);
      const decoded = base64ToArrayBuffer(base64);
      const result = new Uint8Array(decoded);

      expect(result).toEqual(original);
    });

    it("should handle binary data with all byte values", () => {
      const original = new Uint8Array(256);
      for (let i = 0; i < 256; i++) {
        original[i] = i;
      }
      const buffer = original.buffer;

      const base64 = arrayBufferToBase64(buffer);
      const decoded = base64ToArrayBuffer(base64);
      const result = new Uint8Array(decoded);

      expect(result).toEqual(original);
    });
  });

  describe("calculateChunks", () => {
    it("should calculate correct number of chunks for exact division", () => {
      const fileSize = 100 * 1024 * 1024; // 100MB
      const chunkSize = 10 * 1024 * 1024; // 10MB

      const chunks = calculateChunks(fileSize, chunkSize);

      expect(chunks).toBe(10);
    });

    it("should round up for non-exact division", () => {
      const fileSize = 105 * 1024 * 1024; // 105MB
      const chunkSize = 10 * 1024 * 1024; // 10MB

      const chunks = calculateChunks(fileSize, chunkSize);

      expect(chunks).toBe(11); // 10 full chunks + 1 partial
    });

    it("should handle small files (single chunk)", () => {
      const fileSize = 5 * 1024 * 1024; // 5MB
      const chunkSize = 10 * 1024 * 1024; // 10MB

      const chunks = calculateChunks(fileSize, chunkSize);

      expect(chunks).toBe(1);
    });

    it("should handle 5GB file with 10MB chunks", () => {
      const fileSize = 5 * 1024 * 1024 * 1024; // 5GB
      const chunkSize = 10 * 1024 * 1024; // 10MB

      const chunks = calculateChunks(fileSize, chunkSize);

      expect(chunks).toBe(512);
    });
  });

  describe("getFileChunk", () => {
    it("should extract first chunk correctly", async () => {
      const data = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
      const file = new File([data], "test.bin");
      const chunkSize = 4;

      const chunk = await getFileChunk(file, 0, chunkSize);
      const result = new Uint8Array(await chunk.arrayBuffer());

      expect(result).toEqual(new Uint8Array([1, 2, 3, 4]));
    });

    it("should extract middle chunk correctly", async () => {
      const data = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
      const file = new File([data], "test.bin");
      const chunkSize = 4;

      const chunk = await getFileChunk(file, 1, chunkSize);
      const result = new Uint8Array(await chunk.arrayBuffer());

      expect(result).toEqual(new Uint8Array([5, 6, 7, 8]));
    });

    it("should extract last partial chunk correctly", async () => {
      const data = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
      const file = new File([data], "test.bin");
      const chunkSize = 4;

      const chunk = await getFileChunk(file, 2, chunkSize);
      const result = new Uint8Array(await chunk.arrayBuffer());

      expect(result).toEqual(new Uint8Array([9, 10]));
    });

    it("should handle single chunk file", async () => {
      const data = new Uint8Array([1, 2, 3]);
      const file = new File([data], "test.bin");
      const chunkSize = 10;

      const chunk = await getFileChunk(file, 0, chunkSize);
      const result = new Uint8Array(await chunk.arrayBuffer());

      expect(result).toEqual(data);
    });
  });

  describe("calculateChunkHash", () => {
    it("should calculate SHA-256 hash of chunk", async () => {
      const data = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk = new Blob([data]);

      const hash = await calculateChunkHash(chunk);

      expect(typeof hash).toBe("string");
      expect(hash.length).toBeGreaterThan(0);
    });

    it("should produce same hash for same data", async () => {
      const data = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk1 = new Blob([data]);
      const chunk2 = new Blob([data]);

      const hash1 = await calculateChunkHash(chunk1);
      const hash2 = await calculateChunkHash(chunk2);

      expect(hash1).toBe(hash2);
    });

    it("should produce different hash for different data", async () => {
      const data1 = new Uint8Array([1, 2, 3, 4, 5]);
      const data2 = new Uint8Array([1, 2, 3, 4, 6]);
      const chunk1 = new Blob([data1]);
      const chunk2 = new Blob([data2]);

      const hash1 = await calculateChunkHash(chunk1);
      const hash2 = await calculateChunkHash(chunk2);

      expect(hash1).not.toBe(hash2);
    });

    it("should handle empty chunk", async () => {
      const data = new Uint8Array([]);
      const chunk = new Blob([data]);

      const hash = await calculateChunkHash(chunk);

      expect(typeof hash).toBe("string");
      expect(hash.length).toBeGreaterThan(0);
    });

    it("should hash encrypted chunks correctly", async () => {
      const iv = new Uint8Array(12).fill(1);
      const ciphertext = new Uint8Array(100).fill(2);
      const encryptedData = new Uint8Array([...iv, ...ciphertext]);
      const encryptedChunk = new Blob([encryptedData]);

      const hash = await calculateChunkHash(encryptedChunk);

      expect(typeof hash).toBe("string");
      expect(hash.length).toBeGreaterThan(0);
    });
  });

  describe("End-to-End Chunk Processing", () => {
    it("should process file into chunks with hashes", async () => {
      const data = new Uint8Array(30);
      for (let i = 0; i < 30; i++) {
        data[i] = i;
      }
      const file = new File([data], "test.bin");
      const chunkSize = 10;

      const totalChunks = calculateChunks(file.size, chunkSize);
      expect(totalChunks).toBe(3);

      const chunks = [];
      const hashes = [];

      for (let i = 0; i < totalChunks; i++) {
        const chunk = await getFileChunk(file, i, chunkSize);
        const hash = await calculateChunkHash(chunk);

        chunks.push(chunk);
        hashes.push(hash);
      }

      expect(chunks.length).toBe(3);
      expect(hashes.length).toBe(3);

      expect(chunks[0].size).toBe(10);
      expect(chunks[1].size).toBe(10);
      expect(chunks[2].size).toBe(10);

      const uniqueHashes = new Set(hashes);
      expect(uniqueHashes.size).toBe(3);
    });
  });
});
