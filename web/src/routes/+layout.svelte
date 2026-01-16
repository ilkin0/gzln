<script lang="ts">
	import '../app.css';
	import Header from '$lib/components/Header.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();

	const siteTitle = 'GZLN - Simple & Secure File Sharing';
	const siteDescription = 'Fast, secure, and easy file sharing. Upload and share files instantly with GZLN.';
	const siteUrl = 'https://gzln.dev';
	const siteImage = '/og-image.png';
</script>

<svelte:head>
	<!-- Primary Meta Tags -->
	<title>{siteTitle}</title>
	<meta name="title" content={siteTitle} />
	<meta name="description" content={siteDescription} />
	<meta name="keywords" content="file sharing, file upload, secure file transfer, share files online" />
	<meta name="author" content="GZLN" />

	<!-- Favicon & PWA -->
	<link rel="icon" href={favicon} />
	<link rel="apple-touch-icon" href={favicon} />
	<link rel="manifest" href="/manifest.json" />

	<!-- Open Graph / Facebook -->
	<meta property="og:type" content="website" />
	<meta property="og:url" content={siteUrl} />
	<meta property="og:title" content={siteTitle} />
	<meta property="og:description" content={siteDescription} />
	<meta property="og:image" content={siteImage} />
	<meta property="og:site_name" content="GZLN" />

	<!-- Twitter -->
	<meta property="twitter:card" content="summary_large_image" />
	<meta property="twitter:url" content={siteUrl} />
	<meta property="twitter:title" content={siteTitle} />
	<meta property="twitter:description" content={siteDescription} />
	<meta property="twitter:image" content={siteImage} />

	<!-- Canonical URL -->
	<link rel="canonical" href={siteUrl} />
</svelte:head>

<div class="min-h-screen flex flex-col">
	<Header />

	<main class="flex-grow">
		{@render children?.()}
	</main>

	<Footer />
</div>
