<script lang="ts">
  import FileUploader from "$lib/components/FileUploader.svelte";

  const structuredData = {
    "@context": "https://schema.org",
    "@type": "WebApplication",
    name: "GZLN",
    description:
      "Zero-knowledge encrypted file sharing. Your files, your control.",
    url: "https://gzln.dev",
    applicationCategory: "UtilityApplication",
    operatingSystem: "Any",
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
    },
    featureList: [
      "End-to-end encryption",
      "Zero-knowledge architecture",
      "Temporary file storage",
      "No registration required",
    ],
  };
</script>

<svelte:head>
  <title>GZLN - Secure File Sharing</title>
  {@html `<script type="application/ld+json">${JSON.stringify(structuredData)}</script>`}
</svelte:head>

<div class="container">
  <div class="hero">
    <h1>GZLN</h1>
    <p class="tagline">Zero-Knowledge File Sharing</p>
    <p class="description">
      Your files are encrypted in your browser before upload.
      <br />
      We can't see your data. Ever.
    </p>
  </div>

  <FileUploader />

  <div class="features">
    <div class="feature">
      <h3>🔒 End-to-End Encrypted</h3>
      <p>Files are encrypted in your browser using AES-256</p>
    </div>
    <div class="feature">
      <h3>⏱️ Temporary Storage</h3>
      <p>Files automatically expire after download or time limit</p>
    </div>
    <div class="feature">
      <h3>🚀 No Registration</h3>
      <p>Just upload and share. No accounts needed.</p>
    </div>
  </div>
</div>

<style>
  .container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2rem;
  }

  .hero {
    text-align: center;
    margin-bottom: 3rem;
  }

  h1 {
    font-size: 4rem;
    font-weight: bold;
    margin-bottom: 1rem;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .tagline {
    font-size: 1.5rem;
    color: #4a5568;
    margin-bottom: 0.5rem;
  }

  .description {
    color: #718096;
    line-height: 1.6;
  }

  .features {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 2rem;
    margin-top: 4rem;
  }

  .feature {
    text-align: center;
    padding: 1.5rem;
    background: #f7fafc;
    border-radius: 8px;
  }

  .feature h3 {
    font-size: 1.25rem;
    margin-bottom: 0.5rem;
  }

  .feature p {
    color: #718096;
  }
</style>
