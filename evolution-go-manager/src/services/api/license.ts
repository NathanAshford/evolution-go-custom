import apiClient from './client';

export interface LicenseStatus {
  status: 'active' | 'inactive';
  instance_id?: string;
  api_key?: string;
  register_url?: string;
}

export interface RegisterResponse {
  status: string;
  register_url?: string;
  message?: string;
}

export interface ActivateResponse {
  status: string;
  message?: string;
  error?: string;
}

export async function checkLicenseStatus(apiUrl?: string, apiKey?: string): Promise<LicenseStatus> {
  const url = apiUrl ? `${apiUrl}/license/status` : '/license/status';
  const headers: Record<string, string> = {};
  if (apiKey) headers.apikey = apiKey;
  const response = await apiClient.get<LicenseStatus>(url, { headers });
  return response.data;
}

export async function initRegister(redirectUri: string, apiUrl?: string, apiKey?: string): Promise<RegisterResponse> {
  const url = apiUrl ? `${apiUrl}/license/register` : '/license/register';
  const headers: Record<string, string> = {};
  if (apiKey) headers.apikey = apiKey;
  const response = await apiClient.get<RegisterResponse>(url, { headers, params: { redirect_uri: redirectUri } });
  return response.data;
}

export async function activateLicense(code: string, apiUrl?: string, apiKey?: string): Promise<ActivateResponse> {
  const url = apiUrl ? `${apiUrl}/license/activate` : '/license/activate';
  const headers: Record<string, string> = {};
  if (apiKey) headers.apikey = apiKey;
  const response = await apiClient.get<ActivateResponse>(url, { headers, params: { code } });
  return response.data;
}
