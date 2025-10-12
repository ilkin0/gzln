<script lang="ts">
	let files: FileList | null = $state(null);
	let uploading = $state(false);
	let uploadResult: { file_id: string; file_name: string; url: string } | null = $state(null);
	let error = $state('');

	const structuredData = {
		'@context': 'https://schema.org',
		'@type': 'WebApplication',
		name: 'GZLN',
		description: 'Fast, secure, and easy file sharing. Upload and share files instantly.',
		url: 'https://gzln.io',
		applicationCategory: 'UtilityApplication',
		operatingSystem: 'Any',
		offers: {
			'@type': 'Offer',
			price: '0',
			priceCurrency: 'USD'
		},
		featureList: [
			'Secure file upload',
			'Instant file sharing',
			'No registration required',
			'Fast file transfer'
		]
	};

	async function handleUpload() {
		if (!files || files.length === 0) {
			error = 'Please select a file';
			return;
		}

		uploading = true;
		error = '';
		uploadResult = null;

		const formData = new FormData();
		formData.append('file', files[0]);

		try {
			const response = await fetch('/api/files/upload', {
				method: 'POST',
				body: formData
			});

			if (!response.ok) {
				throw new Error('Upload failed');
			}

			const data = await response.json();
			uploadResult = data;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Upload failed';
		} finally {
			uploading = false;
		}
	}

	function resetForm() {
		files = null;
		uploadResult = null;
		error = '';
	}
</script>

<svelte:head>
	{@html `<script type="application/ld+json">${JSON.stringify(structuredData)}</script>`}
</svelte:head>

<div class="bg-gradient-to-br from-blue-50 to-indigo-100 min-h-[calc(100vh-128px)] flex items-center justify-center py-12 px-4">
	<div class="max-w-2xl w-full">
		<!-- Hero Section -->
		<div class="text-center mb-12">
			<h1 class="text-6xl font-bold text-gray-900 mb-4">GZLN</h1>
			<p class="text-2xl text-gray-600 mb-2">Simple & Secure File Sharing</p>
			<p class="text-gray-500">Upload and share files instantly. No registration required.</p>
		</div>

		<!-- Upload Card -->
		<div class="bg-white rounded-2xl shadow-xl p-8">
			{#if uploadResult}
				<!-- Success State -->
				<div class="text-center">
					<div class="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
						<svg class="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
					</div>
					<h2 class="text-2xl font-bold text-gray-900 mb-2">Upload Successful!</h2>
					<p class="text-gray-600 mb-6">{uploadResult.file_name}</p>

					<div class="bg-gray-50 rounded-lg p-4 mb-6">
						<p class="text-sm text-gray-600 mb-2">Download URL:</p>
						<code class="text-sm text-blue-600 break-all">
							{window.location.origin}{uploadResult.url}
						</code>
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
					<label class="block mb-4">
						<div class="border-2 border-dashed border-gray-300 rounded-lg p-12 text-center hover:border-blue-400 transition-colors cursor-pointer">
							<svg class="w-12 h-12 text-gray-400 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
							</svg>

							{#if files && files.length > 0}
								<p class="text-lg font-medium text-gray-900">{files[0].name}</p>
								<p class="text-sm text-gray-500">{(files[0].size / 1024 / 1024).toFixed(2)} MB</p>
							{:else}
								<p class="text-lg font-medium text-gray-700 mb-1">Click to select a file</p>
								<p class="text-sm text-gray-500">or drag and drop</p>
							{/if}
						</div>
						<input
							type="file"
							bind:files
							class="hidden"
						/>
					</label>

					{#if error}
						<div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg">
							<p class="text-red-600 text-sm">{error}</p>
						</div>
					{/if}

					<button
						onclick={handleUpload}
						disabled={!files || uploading}
						class="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
					>
						{uploading ? 'Uploading...' : 'Upload File'}
					</button>
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<p class="text-center text-gray-600 mt-6 text-sm">
			Max file size: 10 MB
		</p>
	</div>
</div>
