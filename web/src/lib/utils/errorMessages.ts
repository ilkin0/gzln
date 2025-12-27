export function getUploadErrorMessage(err: unknown): string {
  if (!(err instanceof Error)) {
    return "Upload failed. Please try again.";
  }

  const message = err.message.toLowerCase();

  if (message.includes("failed to fetch") || message.includes("network")) {
    return "Network error. Please check your internet connection and try again.";
  }

  if (message.includes("404")) {
    return "Upload service is temporarily unavailable. Please try again later.";
  }

  if (
    message.includes("500") ||
    message.includes("502") ||
    message.includes("503")
  ) {
    return "Server error occurred. Please try again in a few moments.";
  }

  if (message.includes("401") || message.includes("403")) {
    return "Upload session expired. Please try again.";
  }

  if (message.includes("413") || message.includes("too large")) {
    return "File is too large. Maximum file size is 5 GB.";
  }

  if (message.includes("timeout") || message.includes("timed out")) {
    return "Upload is taking too long. Please check your connection and try again.";
  }

  if (message.includes("chunk upload failed")) {
    return "Upload was interrupted. Please try again.";
  }

  return "Upload failed. Please try again.";
}

export function getDownloadErrorMessage(err: unknown): string {
  if (!(err instanceof Error)) {
    return "Failed to load file information";
  }

  const message = err.message.toLowerCase();

  if (message.includes("404") || message.includes("not found")) {
    return "This file doesn't exist or has expired";
  }

  if (message.includes("failed to fetch") || message.includes("network")) {
    return "Network error. Please check your connection.";
  }

  if (message.includes("decrypt")) {
    return "Failed to decrypt file information. The link may be corrupted.";
  }

  return "Failed to load file. Please try again.";
}
