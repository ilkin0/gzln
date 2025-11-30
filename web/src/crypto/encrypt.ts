import {
  arrayBufferToBase64,
  base64ToArrayBuffer,
  generateSalt,
} from "./utils";

const PBKDF2_ITERATIONS = 100_000;
const ENCRYPTION_ALGORITHM = "AES-GCM";

export async function generateSecureKey(
  //password: string,
  //salt: string,
): Promise<CryptoKey> {
  const encoder = new TextEncoder();
  const password = crypto.getRandomValues(new Uint8Array(16))
  const salt = generateSalt()

  const passwordBuffer = encoder.encode(password);
  const saltBuffer = base64ToArrayBuffer(salt);

  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    passwordBuffer,
    "PBKDF2",
    false,
    ["deriveKey"],
  );

  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: saltBuffer,
      iterations: PBKDF2_ITERATIONS,
      hash: "SHA-256",
    },
    keyMaterial,
    { name: ENCRYPTION_ALGORITHM, length: 256 },
    false,
    ["encrypt", "decrypt"],
  );
}

export async function encryptString(
  text: string,
  key: CryptoKey,
): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(text);

  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encrypted = await crypto.subtle.encrypt(
    { name: ENCRYPTION_ALGORITHM, iv },
    key,
    data,
  );

  const result = new Uint8Array(iv.length + encrypted.byteLength);
  result.set(iv, 0);
  result.set(new Uint8Array(encrypted), iv.length);

  return arrayBufferToBase64(result.buffer);
}

export async function decryptString(
  encryptedBase64: string,
  key: CryptoKey,
): Promise<string> {
  const encrypted = base64ToArrayBuffer(encryptedBase64);
  const data = new Uint8Array(encrypted);

  const iv = data.slice(0, 12);
  const ciphertext = data.slice(12);

  const decrypted = await crypto.subtle.decrypt(
    { name: ENCRYPTION_ALGORITHM, iv },
    key,
    ciphertext,
  );

  const decoder = new TextDecoder();
  return decoder.decode(decrypted);
}

export async function encryptChunk(chunk: Blob, key: CryptoKey): Promise<Blob> {
  const data = await chunk.arrayBuffer();
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const encrypted = await crypto.subtle.encrypt(
    { name: ENCRYPTION_ALGORITHM, iv },
    key,
    data,
  );

  const result = new Uint8Array(iv.length + encrypted.byteLength);
  result.set(iv, 0);
  result.set(new Uint8Array(encrypted), iv.length);

  return new Blob([result]);
}

export async function decryptChunk(
  encryptedChunk: Blob,
  key: CryptoKey,
): Promise<Blob> {
  const data = await encryptedChunk.arrayBuffer();
  const encrypted = new Uint8Array(data);

  const iv = encrypted.slice(0, 12);
  const ciphertext = encrypted.slice(12);

  const decrypted = await crypto.subtle.decrypt(
    { name: ENCRYPTION_ALGORITHM, iv },
    key,
    ciphertext,
  );

  return new Blob([decrypted]);
}

export { generateSalt, PBKDF2_ITERATIONS };
