import { describe, it, expect } from "vitest";
import {
  deriveKey,
  encryptString,
  decryptString,
  encryptChunk,
  decryptChunk,
} from "./encrypt";
import { generateSalt } from "./utils";

describe("Encryption Functions", () => {
  const TEST_PASSWORD = "test-password-123";

  describe("deriveKey", () => {
    it("should derive a key from password and salt", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      expect(key).toBeInstanceOf(CryptoKey);
      expect(key.type).toBe("secret");
      expect(key.algorithm.name).toBe("AES-GCM");
    });

    it("should derive the same key for same password and salt", async () => {
      const salt = generateSalt();
      const key1 = await deriveKey(TEST_PASSWORD, salt);
      const key2 = await deriveKey(TEST_PASSWORD, salt);

      expect(key1.type).toBe(key2.type);
      expect(key1.algorithm).toEqual(key2.algorithm);
    });

    it("should derive different keys for different salts", async () => {
      const salt1 = generateSalt();
      const salt2 = generateSalt();

      const key1 = await deriveKey(TEST_PASSWORD, salt1);
      const key2 = await deriveKey(TEST_PASSWORD, salt2);

      const testData = "test data";
      const encrypted1 = await encryptString(testData, key1);

      await expect(decryptString(encrypted1, key2)).rejects.toThrow();
    });
  });

  describe("String Encryption/Decryption", () => {
    it("should encrypt and decrypt a string successfully", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalText = "Hello, World!";

      const encrypted = await encryptString(originalText, key);
      const decrypted = await decryptString(encrypted, key);

      expect(decrypted).toBe(originalText);
      expect(encrypted).not.toBe(originalText);
    });

    it("should produce different ciphertext for same plaintext (unique IV)", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalText = "Same text";

      const encrypted1 = await encryptString(originalText, key);
      const encrypted2 = await encryptString(originalText, key);

      expect(encrypted1).not.toBe(encrypted2);

      expect(await decryptString(encrypted1, key)).toBe(originalText);
      expect(await decryptString(encrypted2, key)).toBe(originalText);
    });

    it("should handle empty strings", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalText = "";

      const encrypted = await encryptString(originalText, key);
      const decrypted = await decryptString(encrypted, key);

      expect(decrypted).toBe(originalText);
    });

    it("should handle unicode characters", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalText = "🔐 Encryption test with émojis and spëcial çhars";

      const encrypted = await encryptString(originalText, key);
      const decrypted = await decryptString(encrypted, key);

      expect(decrypted).toBe(originalText);
    });

    it("should fail to decrypt with wrong key", async () => {
      const salt1 = generateSalt();
      const salt2 = generateSalt();
      const key1 = await deriveKey(TEST_PASSWORD, salt1);
      const key2 = await deriveKey(TEST_PASSWORD, salt2);
      const originalText = "Secret message";

      const encrypted = await encryptString(originalText, key1);

      await expect(decryptString(encrypted, key2)).rejects.toThrow();
    });
  });

  describe("Chunk Encryption/Decryption", () => {
    it("should encrypt and decrypt a blob chunk successfully", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalData = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
      const chunk = new Blob([originalData]);

      const encryptedChunk = await encryptChunk(chunk, key);
      const decryptedChunk = await decryptChunk(encryptedChunk, key);

      const decryptedData = new Uint8Array(await decryptedChunk.arrayBuffer());
      expect(decryptedData).toEqual(originalData);
    });

    it("should produce larger encrypted chunk (IV + ciphertext)", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalData = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk = new Blob([originalData]);

      const encryptedChunk = await encryptChunk(chunk, key);

      expect(encryptedChunk.size).toBeGreaterThan(chunk.size);
      expect(encryptedChunk.size).toBe(chunk.size + 12 + 16); // IV + GCM tag
    });

    it("should handle large chunk (simulate 5MB chunk)", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const chunkSize = 5 * 1024 * 1024;
      const originalData = new Uint8Array(chunkSize);
      for (let i = 0; i < chunkSize; i++) {
        originalData[i] = i % 256;
      }
      const chunk = new Blob([originalData]);

      const encryptedChunk = await encryptChunk(chunk, key);
      const decryptedChunk = await decryptChunk(encryptedChunk, key);

      const decryptedData = new Uint8Array(await decryptedChunk.arrayBuffer());

      expect(decryptedData.length).toBe(originalData.length);
      expect(decryptedData[0]).toBe(originalData[0]);
      expect(decryptedData[chunkSize - 1]).toBe(originalData[chunkSize - 1]);
    });

    it("should produce different ciphertext for same chunk (unique IV per chunk)", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);
      const originalData = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk = new Blob([originalData]);

      const encrypted1 = await encryptChunk(chunk, key);
      const encrypted2 = await encryptChunk(chunk, key);

      const data1 = new Uint8Array(await encrypted1.arrayBuffer());
      const data2 = new Uint8Array(await encrypted2.arrayBuffer());

      expect(data1).not.toEqual(data2);

      const decrypted1 = await decryptChunk(encrypted1, key);
      const decrypted2 = await decryptChunk(encrypted2, key);

      const result1 = new Uint8Array(await decrypted1.arrayBuffer());
      const result2 = new Uint8Array(await decrypted2.arrayBuffer());

      expect(result1).toEqual(originalData);
      expect(result2).toEqual(originalData);
    });

    it("should fail to decrypt chunk with wrong key", async () => {
      const salt1 = generateSalt();
      const salt2 = generateSalt();
      const key1 = await deriveKey(TEST_PASSWORD, salt1);
      const key2 = await deriveKey(TEST_PASSWORD, salt2);
      const originalData = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk = new Blob([originalData]);

      const encryptedChunk = await encryptChunk(chunk, key1);

      await expect(decryptChunk(encryptedChunk, key2)).rejects.toThrow();
    });
  });

  describe("End-to-End File Upload Simulation", () => {
    it("should simulate full encryption flow: metadata + chunks", async () => {
      const salt = generateSalt();
      const key = await deriveKey(TEST_PASSWORD, salt);

      const filename = "test-document.pdf";
      const mimeType = "application/pdf";
      const encryptedFilename = await encryptString(filename, key);
      const encryptedMimeType = await encryptString(mimeType, key);

      const chunk1Data = new Uint8Array([1, 2, 3, 4, 5]);
      const chunk2Data = new Uint8Array([6, 7, 8, 9, 10]);
      const chunk1 = new Blob([chunk1Data]);
      const chunk2 = new Blob([chunk2Data]);

      const encryptedChunk1 = await encryptChunk(chunk1, key);
      const encryptedChunk2 = await encryptChunk(chunk2, key);

      const downloadKey = await deriveKey(TEST_PASSWORD, salt);

      const decryptedFilename = await decryptString(
        encryptedFilename,
        downloadKey,
      );
      const decryptedMimeType = await decryptString(
        encryptedMimeType,
        downloadKey,
      );

      expect(decryptedFilename).toBe(filename);
      expect(decryptedMimeType).toBe(mimeType);

      const decryptedChunk1 = await decryptChunk(encryptedChunk1, downloadKey);
      const decryptedChunk2 = await decryptChunk(encryptedChunk2, downloadKey);

      const result1 = new Uint8Array(await decryptedChunk1.arrayBuffer());
      const result2 = new Uint8Array(await decryptedChunk2.arrayBuffer());

      expect(result1).toEqual(chunk1Data);
      expect(result2).toEqual(chunk2Data);
    });
  });
});
