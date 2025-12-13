<script lang="ts">
  import FileDownloader from "$lib/components/FileDownloader.svelte";
  import { page } from "$app/stores";

  const shareId = $derived($page.params.share_id || "");
  let decryptionKey = $state("");
  $effect(() => {
    const hash = window.location.hash;
    if (hash && hash.length > 1) {
      decryptionKey = hash.substring(1);
    }
  });
</script>

<svelte:head>
  <title>Download File - GZLN</title>
  <meta name="description" content="Download your encrypted file securely" />
</svelte:head>

<div class="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center p-4">
  <div class="w-full max-w-2xl">
    {#if !decryptionKey}
      <!-- Missing Key Error -->
      <div class="bg-white rounded-2xl shadow-xl p-8">
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
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div class="flex-1">
              <h3 class="text-sm font-semibold text-red-900 mb-1">Invalid Download Link</h3>
              <p class="text-sm text-red-700">
                This download link is incomplete or invalid. The decryption key is missing.
              </p>
              <p class="text-xs text-red-600 mt-2">
                Please make sure you copied the entire link including the part after the # symbol.
              </p>
            </div>
          </div>
        </div>
      </div>
    {:else}
      <FileDownloader {shareId} {decryptionKey} />
    {/if}
  </div>
</div>
