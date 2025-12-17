import {filesApi} from "$lib/api/files";
import {deriveKey, decryptString} from "../../crypto/encrypt";
import type {FileMetadata} from "$lib/types/api";

export interface DecryptedFileMetadata extends FileMetadata {
    decryptedFilename: string;
    decryptedMimeType: string;
    derivedKey: CryptoKey;
}

export async function loadAndDecryptMetadata(
    shareId: string,
    decryptionKey: string
): Promise<DecryptedFileMetadata> {
    const metadata = await filesApi.getFileMetadata(shareId);
    const derivedKey = await deriveKey(decryptionKey, metadata.salt);

    const decryptedFilename = await decryptString(metadata.encrypted_filename, derivedKey);
    const decryptedMimeType = await decryptString(metadata.encrypted_mime_type, derivedKey);

    return {
        ...metadata,
        decryptedFilename,
        decryptedMimeType,
        derivedKey,
    };
}
