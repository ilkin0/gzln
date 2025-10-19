<script lang="ts">
  import type { UploadProgress } from "$lib/services/chunkUploader";
  import { formatBytes, formatSpeed, formatTime } from "$lib/utils/formatter";

  interface Props {
    progress: UploadProgress;
  }

  let { progress }: Props = $props();

  const percentComplete = $derived(
    Math.round((progress.uploadedBytes / progress.totalBytes) * 100),
  );
</script>

<div class="bg-blue-50 rounded-lg p-6 border border-blue-200">
  <!-- Progress Bar -->
  <div class="mb-4">
    <div class="flex justify-between items-center mb-2">
      <span class="text-sm font-medium text-gray-700">Uploading...</span>
      <span class="text-sm font-semibold text-blue-600">{percentComplete}%</span
      >
    </div>
    <div class="w-full bg-gray-200 rounded-full h-3 overflow-hidden">
      <div
        class="bg-blue-600 h-3 rounded-full transition-all duration-300"
        style="width: {percentComplete}%"
      ></div>
    </div>
  </div>

  <!-- Upload Stats -->
  <div class="grid grid-cols-2 gap-4 text-sm">
    <div>
      <p class="text-gray-600">Progress</p>
      <p class="font-semibold text-gray-900">
        {formatBytes(progress.uploadedBytes)} / {formatBytes(progress.totalBytes)}
      </p>
    </div>
    <div>
      <p class="text-gray-600">Chunks</p>
      <p class="font-semibold text-gray-900">
        {progress.uploadedChunks} / {progress.totalChunks}
      </p>
    </div>
    <div>
      <p class="text-gray-600">Speed</p>
      <p class="font-semibold text-gray-900">
        {formatSpeed(progress.currentSpeed)}
      </p>
    </div>
    <div>
      <p class="text-gray-600">Time Remaining</p>
      <p class="font-semibold text-gray-900">
        {formatTime(progress.estimatedTimeRemaining)}
      </p>
    </div>
  </div>
</div>
