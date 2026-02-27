import type { ApiConfigData, ApiConfigPostBody, ApiEnvelope } from '../types/configApi';

type ApiResult = {
  success: boolean;
  reason: string;
};

type WindowWithApiBase = Window & {
  __BLACKBOX_API_BASE_URL__?: string;
};

function trimTrailingSlashes(value: string): string {
  return value.replace(/\/+$/, '');
}

function inferBaseFromPath(pathname: string): string {
  const appMatch = pathname.match(/^\/web\/apps\/[^/]+/);
  if (appMatch) {
    return appMatch[0];
  }

  if (pathname.startsWith('/web/')) {
    return '/web';
  }

  return '';
}

function apiBaseUrl(): string {
  const runtimeBase = (window as WindowWithApiBase).__BLACKBOX_API_BASE_URL__?.trim() ?? '';
  const envBase = ((import.meta as ImportMeta & { env: Record<string, string | undefined> }).env
    .VITE_CONFIG_API_BASE_URL ?? ''
  ).trim();
  const inferredBase = inferBaseFromPath(window.location.pathname);
  const candidate = runtimeBase || envBase || inferredBase;
  return trimTrailingSlashes(candidate);
}

function apiUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  const base = apiBaseUrl();
  if (!base) {
    return normalizedPath;
  }
  return `${base}${normalizedPath}`;
}

function parseEnvelope<T>(raw: unknown): ApiEnvelope<T> {
  if (!raw || typeof raw !== 'object') {
    throw new Error('Invalid API response');
  }
  const envelope = raw as ApiEnvelope<T>;
  if (typeof envelope.success !== 'boolean') {
    throw new Error('Invalid API response: missing success');
  }
  return envelope;
}

async function parseJsonResponse<T>(response: Response): Promise<ApiEnvelope<T>> {
  const body: unknown = await response.json();
  const envelope = parseEnvelope<T>(body);
  if (!response.ok) {
    throw new Error(envelope.reason || `HTTP ${response.status}`);
  }
  if (!envelope.success) {
    throw new Error(envelope.reason || 'Request failed');
  }
  return envelope;
}

export async function getConfig(): Promise<ApiConfigData> {
  const response = await fetch(apiUrl('/api/config'), {
    method: 'GET',
  });
  const envelope = await parseJsonResponse<ApiConfigData>(response);
  return envelope.data;
}

export async function postConfig(payload: ApiConfigPostBody): Promise<ApiResult> {
  const response = await fetch(apiUrl('/api/config'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  const envelope = await parseJsonResponse<unknown>(response);
  return {
    success: envelope.success,
    reason: envelope.reason || '',
  };
}
