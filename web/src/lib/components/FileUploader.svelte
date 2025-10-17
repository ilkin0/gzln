<script lang="ts">
  import { filesApi } from "$lib/api/files";
  import { MAX_FILE_SIZE, CHUNK_SIZE } from "$lib/config";
  import type { InitUploadRequest } from "$lib/types/api";

  let files: FileList | null = $state(null);
  let uploading = $state(false);
  let uploadResult: {
    share_id: string;
    file_id: string;
    expires_at: string;
  } | null = $state(null);
  let error = $state("");
  let copied = $state(false);
  let isDragging = $state(false);

  const TEMP_PASSWORD = "temp-password-placeholder";

  // Auto-upload when file is selected
  $effect(() => {
    if (files && files.length > 0 && !uploading && !uploadResult) {
      handleUpload();
    }
  });

  async function handleUpload() {
    if (!files || files.length === 0) {
      error = "Please select a file";
      return;
    }

    const file = files[0];

    if (file.size > MAX_FILE_SIZE) {
      error = `File size exceeds ${MAX_FILE_SIZE / (1024 * 1024 * 1024)}GB limit`;
      return;
    }

    uploading = true;
    error = "";
    uploadResult = null;

    try {
      const salt = "generate-random-salt";

      const key = "derive_Key"; // TODO

      const encryptedFilename = "encrypted_string";
      const encryptedMimeType = "encrypt_string";

      const chunkCount = 100;

      const request: InitUploadRequest = {
        salt,
        encrypted_filename: encryptedFilename,
        encrypted_mime_type: encryptedMimeType,
        total_size: file.size,
        chunk_count: chunkCount,
        chunk_size: CHUNK_SIZE,
        pbkdf2_iterations: 1000, // TODO
      };

      const response = await filesApi.initUpload(request);

      // TODO: Upload actual file chunks

      uploadResult = {
        share_id: response.share_id,
        file_id: response.file_id,
        expires_at: response.expires_at,
      };
    } catch (err) {
      error = err instanceof Error ? err.message : "Upload failed";
    } finally {
      uploading = false;
    }
  }

  function resetForm() {
    files = null;
    uploadResult = null;
    error = "";
    copied = false;
  }

  function copyToClipboard() {
    const url = `${window.location.origin}/${uploadResult?.share_id}`;
    navigator.clipboard.writeText(url);
    copied = true;
    setTimeout(() => {
      copied = false;
    }, 2000);
  }

  function formatDate(dateString: string): string {
    const date = new Date(dateString);
    return date.toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
    });
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    isDragging = true;
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;

    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      const dt = new DataTransfer();
      dt.items.add(e.dataTransfer.files[0]);
      files = dt.files;
    }
  }
</script>

<div class="bg-white rounded-2xl shadow-xl p-8">
  {#if uploadResult}
    <!-- Success State -->
    <div class="text-center">
      <div
        class="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4"
      >
        <svg
          class="w-8 h-8 text-green-600"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 13l4 4L19 7"
          />
        </svg>
      </div>
      <h2 class="text-2xl font-bold text-gray-900 mb-2">Upload Successful!</h2>
      <p class="text-gray-600 mb-6">Share this link to download the file</p>

      <!-- Share Link Section -->
      <div
        class="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-xl p-6 mb-4 border border-blue-200"
      >
        <p
          class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3"
        >
          Download Link
        </p>
        <div class="flex flex-col sm:flex-row gap-3">
          <div
            class="flex-1 flex items-center gap-3 bg-white rounded-lg p-4 border-2 border-blue-200"
          >
            <svg
              class="w-5 h-5 text-blue-600 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
              />
            </svg>
            <code
              class="flex-1 text-sm md:text-base font-mono text-blue-700 break-all"
            >
              {window.location.origin}/{uploadResult.share_id}
            </code>
          </div>
          <button
            onclick={copyToClipboard}
            class="flex items-center justify-center gap-2 px-6 py-4 bg-blue-600 hover:bg-blue-700 text-white font-semibold rounded-lg transition-colors shadow-md hover:shadow-lg flex-shrink-0"
            title="Copy link to clipboard"
          >
            {#if copied}
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
                  d="M5 13l4 4L19 7"
                />
              </svg>
              <span>Copied!</span>
            {:else}
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
                  d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                />
              </svg>
              <span>Copy</span>
            {/if}
          </button>
        </div>
      </div>

      <!-- Expiry Info -->
      <div class="bg-amber-50 rounded-lg p-4 mb-6 border border-amber-200">
        <div class="flex items-start gap-3">
          <svg
            class="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5"
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
            <p class="text-sm font-medium text-amber-900">Link expires on</p>
            <p class="text-sm text-amber-700 font-semibold">
              {formatDate(uploadResult.expires_at)}
            </p>
          </div>
        </div>
      </div>

      <button
        onclick={resetForm}
        class="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
      >
        Upload Another File
      </button>
    </div>
  {:else}
    <!-- Upload Form -->
    <div>
      {#if !uploading}
        <div
          ondragover={handleDragOver}
          ondragleave={handleDragLeave}
          ondrop={handleDrop}
        >
          <label class="block">
            <div
              class="border-2 border-dashed rounded-lg p-12 text-center transition-all cursor-pointer {isDragging
                ? 'border-blue-500 bg-blue-50 scale-105'
                : 'border-gray-300 hover:border-blue-400'}"
            >
              <svg
                class="w-12 h-12 mx-auto mb-4 transition-colors {isDragging
                  ? 'text-blue-500'
                  : 'text-gray-400'}"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                />
              </svg>

              {#if isDragging}
                <p class="text-lg font-medium text-blue-700 mb-1">
                  Drop your file here
                </p>
                <p class="text-sm text-blue-600">Release to upload</p>
              {:else}
                <p class="text-lg font-medium text-gray-700 mb-1">
                  Click to select a file
                </p>
                <p class="text-sm text-gray-500">or drag and drop</p>
                <p class="text-xs text-gray-400 mt-2">Max file size: 5 GB</p>
              {/if}
            </div>
            <input type="file" bind:files class="hidden" />
          </label>
        </div>
      {:else}
        <!-- Uploading State -->
        <div
          class="border-2 border-blue-300 bg-blue-50 rounded-lg p-12 text-center"
        >
          <div class="flex justify-center mb-4">
            <div
              class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"
            ></div>
          </div>
          <p class="text-lg font-medium text-gray-900 mb-1">Uploading...</p>
          {#if files && files[0]}
            <p class="text-sm text-gray-600">{files[0].name}</p>
            <p class="text-xs text-gray-500 mt-1">
              {(files[0].size / 1024 / 1024).toFixed(2)} MB
            </p>
          {/if}
        </div>
      {/if}

      {#if error}
        <div class="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg">
          <p class="text-red-600 text-sm">{error}</p>
          <button
            onclick={resetForm}
            class="mt-2 text-sm text-blue-600 hover:text-blue-700 font-medium"
          >
            Try again
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>
