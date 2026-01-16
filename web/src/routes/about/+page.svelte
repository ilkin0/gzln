<svelte:head>
	<title>About - GZLN</title>
	<meta name="description" content="Learn about GZLN - a simple, secure, and open-source file sharing platform." />
</svelte:head>

<div class="bg-gradient-to-br from-blue-50 to-indigo-100 min-h-[calc(100vh-128px)] py-16 px-4">
	<div class="max-w-4xl mx-auto">
		<!-- Header -->
		<div class="text-center mb-12">
			<h1 class="text-5xl font-bold text-gray-900 mb-4">About GZLN</h1>
			<p class="text-xl text-gray-600">Simple. Secure. Open Source.</p>
		</div>

		<!-- Content Cards -->
		<div class="space-y-6">
			<!-- Mission -->
			<div class="bg-white rounded-2xl shadow-lg p-8">
				<div class="flex items-center mb-4">
					<svg class="w-8 h-8 text-blue-600 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
					</svg>
					<h2 class="text-2xl font-bold text-gray-900">Our Mission</h2>
				</div>
				<p class="text-gray-700 leading-relaxed">
					GZLN is built to make file sharing simple, fast, and secure. No complicated sign-ups,
					no hidden fees, just straightforward file sharing that works. File sharing
					should be accessible to everyone.
				</p>
			</div>

			<!-- Features -->
			<div class="bg-white rounded-2xl shadow-lg p-8">
				<div class="flex items-center mb-4">
					<svg class="w-8 h-8 text-blue-600 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
					<h2 class="text-2xl font-bold text-gray-900">Features</h2>
				</div>
				<ul class="space-y-3">
					<li class="flex items-start">
						<svg class="w-6 h-6 text-green-500 mr-2 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						<span class="text-gray-700"><strong>Instant Upload:</strong> Share files in seconds</span>
					</li>
					<li class="flex items-start">
						<svg class="w-6 h-6 text-green-500 mr-2 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						<span class="text-gray-700"><strong>No Registration:</strong> No account needed</span>
					</li>
					<li class="flex items-start">
						<svg class="w-6 h-6 text-green-500 mr-2 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						<span class="text-gray-700"><strong>Secure Storage:</strong> Files stored safely with MinIO</span>
					</li>
					<li class="flex items-start">
						<svg class="w-6 h-6 text-green-500 mr-2 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						<span class="text-gray-700"><strong>Fast Delivery:</strong> Optimized for speed</span>
					</li>
				</ul>
			</div>

			<!-- Privacy First -->
			<div class="bg-white rounded-2xl shadow-lg p-8">
				<div class="flex items-center mb-4">
					<svg class="w-8 h-8 text-blue-600 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
					</svg>
					<h2 class="text-2xl font-bold text-gray-900">Privacy First</h2>
				</div>
				<p class="text-gray-700 leading-relaxed mb-4">
					Your files are encrypted in your browser before they ever leave your device.
					We use <strong>AES-256-GCM</strong> encryption with keys derived via <strong>PBKDF2-SHA256</strong>.
				</p>
				<div class="bg-gray-50 rounded-lg p-4 text-sm text-gray-600">
					<p class="font-medium text-gray-800 mb-2">How it works:</p>
					<ol class="list-decimal list-inside space-y-1">
						<li>A random encryption key is generated in your browser</li>
						<li>Your file is encrypted locally before upload</li>
						<li>The key stays in the URL fragment — never sent to our server</li>
						<li>Only you and whoever you share the link with can decrypt the file</li>
					</ol>
				</div>
			</div>

			<!-- Open Source -->
			<div class="bg-white rounded-2xl shadow-lg p-8">
				<div class="flex items-center mb-4">
					<svg class="w-8 h-8 text-blue-600 mr-3" fill="currentColor" viewBox="0 0 24 24">
						<path fill-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clip-rule="evenodd" />
					</svg>
					<h2 class="text-2xl font-bold text-gray-900">Open Source</h2>
				</div>
				<p class="text-gray-700 leading-relaxed mb-4">
					GZLN is completely open source. The code is available on GitHub for anyone to review,
					contribute to, or learn from. <br>I believe in transparency and community-driven development.
				</p>
				<a
					href="https://github.com/ilkin0/gzln"
					target="_blank"
					rel="noopener noreferrer"
					class="inline-flex items-center space-x-2 bg-gray-900 hover:bg-gray-800 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
				>
					<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
						<path fill-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clip-rule="evenodd" />
					</svg>
					<span>View on GitHub</span>
				</a>
			</div>
		</div>
	</div>
</div>
