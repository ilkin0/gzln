<script lang="ts">
  import { filesApi } from "$lib/api/files";
  import { deriveKey, decryptString } from "../../crypto/encrypt";
  import { formatBytes } from "$lib/utils/formatter";
  import { isExpired } from "$lib/utils/timeUtils";
  import CountdownTimer from "./CountdownTimer.svelte";
  import type { FileMetadata } from "$lib/types/api";

  interface Props {
    shareId: string;
    decryptionKey: string;
  }

  let { shareId, decryptionKey }: Props = $props();
  console.log(`ShareID: ${shareId}, DepKey: ${decryptionKey}`)

  type PageState = "loading" | "ready" | "downloading" | "error" | "expired";

  let pageState: PageState = $state("loading");
  let metadata: FileMetadata | null = $state(null);
  let decryptedFilename = $state("");
  let decryptedMimeType = $state("");
  let errorMessage = $state("");

  function getUserFriendlyError(err: unknown): string {
    if (!(err instanceof Error)) return "Failed to load file information";

    const message = err.message.toLowerCase();

    if (message.includes("404") || message.includes("not found")) {
      return "This file doesn't exist or has expired";
    }

    if (message.includes("failed to fetch") || message.includes("network")) {
      return "Network error. Please check your connection.";
    }

    if (message.includes("decrypt")) {
      return "Failed to decrypt file information. The link may be corrupted.";
    }

    return "Failed to load file. Please try again.";
  }

  async function loadFileMetadata() {
    pageState = "loading";
    try {
      const data = await filesApi.getMockFileMetadata(shareId);

      if (isExpired(data.expires_at)) {
        pageState = "expired";
        return;
      }

      const key = await deriveKey(decryptionKey, data.salt);
      decryptedFilename = await decryptString(data.encrypted_filename, key);
      decryptedMimeType = await decryptString(data.encrypted_mime_type, key);

      metadata = data;
      pageState = "ready";
    } catch (err) {
      console.error("Failed to load file metadata:", err);
      pageState = "error";
      errorMessage = getUserFriendlyError(err);
    }
  }

  $effect(() => {
    if (shareId && decryptionKey) {
      loadFileMetadata();
    }
  });

  function handleDownload() {
    alert(`Would download: ${decryptedFilename}`);
  }

  function handleExpired() {
    pageState = "expired";
  }
</script>

<div class="bg-white rounded-2xl shadow-xl p-8">
  {#if pageState === "loading"}
    <!-- Loading State -->
    <div class="border-2 border-blue-300 bg-blue-50 rounded-lg p-12 text-center">
      <div class="flex justify-center mb-4">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
      <p class="text-lg font-medium text-gray-900 mb-1">Loading file information...</p>
      <p class="text-sm text-gray-600">Decrypting metadata</p>
    </div>
  {:else if pageState === "error"}
    <!-- Error State -->
    <div class="bg-red-50 border border-red-200 rounded-lg p-6">
      <div class="flex items-start gap-3">
        <div class="flex-shrink-0">
          <svg
            class="w-6 h-6 text-red-600"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </div>
        <div class="flex-1">
          <h3 class="text-sm font-semibold text-red-900 mb-1">Error</h3>
          <p class="text-sm text-red-700">{errorMessage}</p>
        </div>
      </div>
    </div>
  {:else if pageState === "expired"}
    <!-- Expired State -->
    <div class="bg-red-50 border border-red-200 rounded-lg p-6">
      <div class="flex items-start gap-3">
        <svg
          class="w-6 h-6 text-red-600 flex-shrink-0"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <div>
          <h3 class="text-sm font-semibold text-red-900 mb-1">File Expired</h3>
          <p class="text-sm text-red-700">This file has expired and is no longer available for download</p>
        </div>
      </div>
    </div>
  {:else if pageState === "ready" && metadata}
    <!-- Ready State -->
    <div class="text-center">
      <!-- File Icon -->
      <div class="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-4">
        <svg
          class="w-8 h-8 text-blue-600"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
      </div>

      <!-- File Name & Size -->
      <h2 class="text-2xl font-bold text-gray-900 mb-2 break-all">{decryptedFilename}</h2>
      <p class="text-gray-600 mb-6">{formatBytes(metadata.total_size)}</p>

      <!-- Countdown Timer -->
      <div class="mb-4">
        <CountdownTimer expiresAt={metadata.expires_at} onExpired={handleExpired} />
      </div>

      <!-- Download Info -->
      <div class="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-xl p-6 mb-6 border border-blue-200">
        <div class="flex items-center justify-center gap-3">
          <svg
            class="w-5 h-5 text-blue-600"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
          <div>
            <p class="text-sm font-medium text-blue-900">
              {metadata.download_count} of {metadata.max_downloads} downloads used
            </p>
            <p class="text-xs text-blue-700">
              {metadata.max_downloads - metadata.download_count} downloads remaining
            </p>
          </div>
        </div>
      </div>

      <!-- Download Button -->
      <button
        onclick={handleDownload}
        class="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-4 px-6 rounded-lg transition-colors shadow-md hover:shadow-lg flex items-center justify-center gap-2"
      >
        <svg
          class="w-5 h-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
          />
        </svg>
        <span>Download File</span>
      </button>

      <!-- File Details -->
      <div class="mt-6 pt-6 border-t border-gray-200">
        <div class="flex flex-wrap gap-4 justify-center text-sm text-gray-500">
          <div class="flex items-center gap-1">
            <svg
              class="w-4 h-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
            <span>{decryptedMimeType || "Unknown type"}</span>
          </div>
          <div class="flex items-center gap-1">
            <svg
              class="w-4 h-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01"
              />
            </svg>
            <span>{metadata.chunk_count} chunks</span>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
