<script lang="ts">
    import {formatBytes} from "$lib/utils/formatter";
    import {isExpired} from "$lib/utils/timeUtils";
    import CountdownTimer from "./CountdownTimer.svelte";
    import {type DecryptedFileMetadata, loadAndDecryptMetadata} from "$lib/services/fileMetadata";
    import {downloadFileInChunks} from "$lib/services/chunkDownloader";
    import { filesApi } from "$lib/api/files";
    import { getDownloadErrorMessage } from "$lib/utils/errorMessages";

    interface Props {
        shareId: string;
        decryptionKey: string;
    }

    let {shareId, decryptionKey}: Props = $props();

    type PageState = "loading" | "ready" | "downloading" | "error" | "expired" | "exhausted";

    let pageState: PageState = $state("loading");
    let metadata: DecryptedFileMetadata | null = $state(null);
    let errorMessage = $state("");

    let overallProgress = $state(0);
    let chunkProgress: number[] = $state([]);
    let downloadSpeed = $state(0);
    let eta = $state(0);
    let startTime = 0;

    function formatETA(seconds: number): string {
        if (!seconds || seconds === Infinity || isNaN(seconds)) {
            return "calculating...";
        }

        if (seconds < 1) {
            return "< 1s";
        }

        if (seconds < 60) {
            return `${Math.round(seconds)}s`;
        }

        if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60);
            const secs = Math.round(seconds % 60);
            return secs > 0 ? `${minutes}m ${secs}s` : `${minutes}m`;
        }

        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
    }

    async function loadFileMetadata() {
        pageState = "loading";
        try {
            metadata = await loadAndDecryptMetadata(shareId, decryptionKey);

            if (isExpired(metadata.expires_at)) {
                pageState = "expired";
                return;
            }

            if (metadata.download_count >= metadata.max_downloads) {
                pageState = "exhausted";
                return;
            }

            pageState = "ready";
        } catch (err) {
            console.error("Failed to load file metadata:", err);
            pageState = "error";
            errorMessage = getDownloadErrorMessage(err);
        }
    }

    $effect(() => {
        if (shareId && decryptionKey) {
            loadFileMetadata();
        }
    });

    async function handleDownload() {
        if (!metadata) return;

        try {
            pageState = "downloading";
            overallProgress = 0;
            chunkProgress = new Array(metadata.chunk_count).fill(0);
            downloadSpeed = 0;
            eta = 0;
            startTime = Date.now();

            const chunks = await downloadFileInChunks({
                shareId,
                totalChunks: metadata.chunk_count,
                decryptionKey: metadata.derivedKey,
                onProgress: (progress) => {
                    if (!metadata) return;

                    chunkProgress[progress.chunkIndex] = progress.bytesReceived;

                    const totalDownloaded = chunkProgress.reduce((sum, val) => sum + val, 0);
                    const totalSize = metadata.total_size;
                    overallProgress = totalSize > 0 ? (totalDownloaded / totalSize) * 100 : 0;

                    const elapsed = (Date.now() - startTime) / 1000;
                    if (elapsed > 0) {
                        downloadSpeed = totalDownloaded / elapsed;
                    }
                    const remaining = totalSize - totalDownloaded;
                    eta = downloadSpeed > 0 ? remaining / downloadSpeed : 0;
                },
            });

            if (!chunks.length) {
                throw new Error("Nothing to download");
            }

            const blob = new Blob(chunks as BlobPart[], {
                type: metadata.decryptedMimeType || "application/octet-stream"
            });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = metadata.decryptedFilename || "download";
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);

            await filesApi.completeDownload(shareId);
            await loadFileMetadata();
        } catch (err) {
            console.error("Download error:", err);
            pageState = "error";
            errorMessage = getDownloadErrorMessage(err);
        }
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

    {:else if pageState === "downloading"}
        <!-- Downloading State -->
        <div class="border-2 border-blue-300 bg-blue-50 rounded-lg p-12 text-center">
            <!-- Spinner -->
            <div class="mb-6">
                <div class="animate-spin rounded-full h-16 w-16 border-b-4 border-blue-600 mx-auto"></div>
            </div>

            <p class="text-xl font-bold text-gray-900 mb-1">Downloading & Decrypting...</p>

            <p class="text-3xl font-bold text-blue-600 mb-4">
                {overallProgress.toFixed(1)}%
            </p>

            <div class="w-full bg-gray-200 rounded-full h-3 mb-3 overflow-hidden">
                <div
                        class="bg-gradient-to-r from-blue-500 to-blue-600 h-3 rounded-full transition-all duration-500 ease-out"
                        style="width: {Math.min(overallProgress, 100)}%"
                ></div>
            </div>

            <div class="flex justify-between items-center text-sm text-gray-600 px-2">
                <div class="flex items-center gap-1.5">
                    <svg class="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                              d="M13 10V3L4 14h7v7l9-11h-7z"/>
                    </svg>
                    <span class="font-medium">
            {downloadSpeed > 0 ? formatBytes(downloadSpeed) + '/s' : 'calculating...'}
          </span>
                </div>

                <div class="flex items-center gap-1.5">
                    <svg class="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
                    </svg>
                    <span class="font-medium">
            {eta > 0 ? formatETA(eta) + ' left' : 'calculating...'}
          </span>
                </div>
            </div>

            {#if metadata}
                <p class="text-xs text-gray-500 mt-4">
                    {metadata.decryptedFilename}
                </p>
            {/if}
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

    {:else if pageState === "exhausted"}
        <!-- Exhausted State -->
        <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
            <div class="flex items-start gap-3">
                <svg
                        class="w-6 h-6 text-amber-600 flex-shrink-0"
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
                <div>
                    <h3 class="text-sm font-semibold text-amber-900 mb-1">Download Limit Reached</h3>
                    <p class="text-sm text-amber-700">This file has reached its maximum download limit and is no longer available</p>
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
            <h2 class="text-2xl font-bold text-gray-900 mb-2 break-all">{metadata.decryptedFilename}</h2>
            <p class="text-gray-600 mb-6">{formatBytes(metadata.total_size)}</p>

            <!-- Countdown Timer -->
            <div class="mb-4">
                <CountdownTimer expiresAt={metadata.expires_at} onExpired={handleExpired}/>
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
                        <span>{metadata.decryptedMimeType || "Unknown type"}</span>
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
