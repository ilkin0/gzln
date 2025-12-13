<script lang="ts">
  import { page } from "$app/stores";

  const status = $derived($page.status);
  const message = $derived($page.error?.message || "An unexpected error occurred");

  const errorMessages: Record<number, { title: string; description: string }> = {
    404: {
      title: "Page Not Found",
      description: "The page you're looking for doesn't exist or has been moved."
    },
    500: {
      title: "Internal Server Error",
      description: "Something went wrong on our end. Please try again later."
    },
    403: {
      title: "Access Forbidden",
      description: "You don't have permission to access this resource."
    }
  };

  const errorInfo = $derived(
    errorMessages[status] || {
      title: "Oops! Something Went Wrong",
      description: message
    }
  );
</script>

<svelte:head>
  <title>{errorInfo.title} - GZLN</title>
</svelte:head>

<div class="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center p-4">
  <div class="w-full max-w-lg text-center px-4 mb-5">
    <!-- Error Generic Message -->
    <div class="mb-3">
      <span class="text-7xl font-bold text-white/50">Something Went Wrong :(</span>
    </div>

    <!-- Error Title -->
    <h1 class="text-5xl font-bold text-gray-800 mb-2">
      {errorInfo.title}
    </h1>

    <!-- Error Description -->
    <p class="text-gray-700 mb-8">
      {errorInfo.description}
    </p>

    <!-- Actions -->
    <div class="flex flex-col sm:flex-row gap-3 justify-center max-w-sm mx-auto">
      <a
        href="/"
        class="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
      >
        Go to Home
      </a>

<!--      <button-->
<!--        onclick={() => window.history.back()}-->
<!--        class="bg-white/80 backdrop-blur hover:bg-white text-gray-700 font-semibold py-3 px-6 rounded-lg transition-colors"-->
<!--      >-->
<!--        Go Back-->
<!--      </button>-->
    </div>

    <!-- Additional Info (for non-production) -->
    {#if import.meta.env.DEV && message !== errorInfo.description}
      <div class="mt-8 max-w-md mx-auto">
        <details class="text-sm bg-white/60 backdrop-blur rounded-lg p-3">
          <summary class="text-gray-700 cursor-pointer hover:text-gray-900 font-medium">
            Technical Details
          </summary>
          <p class="mt-2 text-xs text-gray-600 font-mono bg-gray-100 p-2 rounded text-left break-all">
            {message}
          </p>
        </details>
      </div>
    {/if}
  </div>
</div>
