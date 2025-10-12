<script lang="ts">
  const githubUrl = "https://github.com/ilkin0/gzln";
  const authorName = "ilkin";
  const authorUrl = "https://github.com/ilkin0";
</script>

<footer class="bg-white mt-auto">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2">
    <div class="flex flex-row items-center justify-center space-x-2">
      <span>Created by </span>
      <a
        href={authorUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="font-medium text-gray-600 hover:text-gray-900 transition-colors"
      >
        {`@${authorName}`}
      </a>
      <p>•</p>
      <a
        href={githubUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="font-medium text-gray-600 hover:text-gray-900 transition-colors"
        aria-label="View source on GitHub"
      >
        <span class="text-sm font-medium">GitHub</span>
      </a>
    </div>
  </div>
</footer>
