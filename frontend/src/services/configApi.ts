import type { ApiConfigData, ApiConfigPostBody, ApiEnvelope } from '../types/configApi';

type ApiResult = {
  success: boolean;
  reason: string;
};

function apiBaseUrl(): string {
  return `${window.location.protocol}//${window.location.hostname}:8000`;
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
  const response = await fetch(`${apiBaseUrl()}/api/config`, {
    method: 'GET',
  });
  const envelope = await parseJsonResponse<ApiConfigData>(response);
  return envelope.data;
}

export async function postConfig(payload: ApiConfigPostBody): Promise<ApiResult> {
  const response = await fetch(`${apiBaseUrl()}/api/config`, {
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
