import {API_BASE_URL} from "$lib/config";
import type {ApiError, ApiResponse} from "$lib/types/api";

class ApiClient {
  private readonly baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;

    try {
      const res = await fetch(url, {
        ...options,
        headers: this.mergeHeaders(options),
      });

      const payload: ApiResponse<T> = await res.json();
      if (!payload?.success || payload.data === undefined) {
        throw {
          message: (payload as unknown as { message?: string })?.message ?? `Request failed`,
          status: res.status,
        } as ApiError;
      }

      return payload.data as T;
    } catch (error) {
      if ((error as ApiError).status !== undefined) {
        throw error;
      }
      // Network or parsing error
      throw {
        message: "Network error. Please check your connection.",
        status: 0,
      } as ApiError;
    }
  }

  get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: "GET" });
  }

  /**
   * GET request that returns the raw Response object
   * Use for binary data, streaming, or custom response handling
   *
   * @param endpoint - API endpoint
   * @returns Promise<Response>
   */
  async getRaw(endpoint: string): Promise<Response> {
    const url = `${this.baseUrl}${endpoint}`;

    try {
      const response = await fetch(url, {
        method: "GET",
        headers: this.mergeHeaders(),
      });

      if (!response.ok) {
        throw {
          message: `Request failed: ${response.statusText}`,
          status: response.status,
        } as ApiError;
      }

      return response;
    } catch (error) {
      if ((error as ApiError).status !== undefined) {
        throw error;
      }
      throw {
        message: "Network error. Please check your connection.",
        status: 0,
      } as ApiError;
    }
  }

  post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: "POST",
      body: data === undefined ? undefined : JSON.stringify(data),
    });
  }

  async postForm<T>(endpoint: string, formData: FormData, extraHeaders?: Record<string, string>): Promise<T> {
    return this.request<T>(endpoint, {
      method: "POST",
      body: formData,
      headers: extraHeaders,
    });
  }

  private mergeHeaders(options?: RequestInit): HeadersInit | undefined {
    const isFormData = options?.body instanceof FormData;
    const base: Record<string, string> = isFormData
      ? {}
      : { "Content-Type": "application/json" };

    // Normalize incoming headers
    let incoming: Record<string, string> = {};
    if (options?.headers) {
      if (options.headers instanceof Headers) {
        options.headers.forEach((v, k) => (incoming[k] = v));
      } else if (Array.isArray(options.headers)) {
        for (const [k, v] of options.headers) incoming[k] = v;
      } else {
        incoming = { ...(options.headers as Record<string, string>) };
      }
    }

    return { ...base, ...incoming };
  }
}

export const apiClient = new ApiClient(API_BASE_URL);
