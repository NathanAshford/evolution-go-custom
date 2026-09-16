/**
 * Instances API Service
 * Handles all Evolution GO instance-related API calls
 */

import apiClient from './client';
import type {
  Instance,
  RawInstance,
  InstancesResponse,
  CreateInstancePayload,
  ConnectionState,
  InstanceStatus,
  ProxyInfo,
} from '@/types/instance';

/**
 * Parse the proxy column, which the API returns as a JSON string (empty when no
 * proxy is configured). Malformed values are treated as "no proxy" rather than
 * breaking the whole instance list.
 */
const parseProxy = (raw: string | undefined): ProxyInfo | undefined => {
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as ProxyInfo;
    return parsed?.host ? parsed : undefined;
  } catch {
    console.warn('Configuração de proxy inválida recebida da API:', raw);
    return undefined;
  }
};

/**
 * Normalize raw instance data from Evolution GO API
 */
const normalizeInstance = (raw: RawInstance): Instance => {
  // Determine status from connected field
  const status: InstanceStatus = raw.connected ? 'open' : 'close';

  // Parse QR code if available
  let qrcode: Instance['qrcode'];
  if (raw.qrcode) {
    const parts = raw.qrcode.split('|');
    qrcode = {
      base64: parts[0] || undefined,
      code: parts[1] || undefined,
    };
  }

  return {
    proxy: parseProxy(raw.proxy),
    id: raw.id,
    instanceName: raw.name,
    status,
    apikey: raw.token,
    owner: raw.jid ? raw.jid.split('@')[0] : '',
    profileName: raw.name,
    connected: raw.connected,
    qrcode,
    webhook: raw.webhook || undefined,
    rabbitmqEnable: raw.rabbitmqEnable || undefined,
    websocketEnable: raw.websocketEnable || undefined,
    natsEnable: raw.natsEnable || undefined,
    events: raw.events || undefined,
    disconnectReason: raw.disconnect_reason || undefined,
    createdAt: raw.createdAt,
    alwaysOnline: raw.alwaysOnline,
    rejectCall: raw.rejectCall,
    readMessages: raw.readMessages,
    ignoreGroups: raw.ignoreGroups,
    ignoreStatus: raw.ignoreStatus,
  };
};

/**
 * Fetch all instances
 * GET /instance/all
 */
export const fetchInstances = async (): Promise<Instance[]> => {
  const response = await apiClient.get<InstancesResponse>('/instance/all');
  // Normalize the instances from Evolution GO format to our format
  return response.data.data.map(normalizeInstance);
};

/**
 * Fetch a single instance by ID
 * GET /instance/info/:instanceId
 */
export const fetchInstance = async (instanceId: string): Promise<Instance> => {
  const response = await apiClient.get<{
    message: string;
    data: RawInstance;
  }>(`/instance/info/${instanceId}`);
  return normalizeInstance(response.data.data);
};

export interface ReachoutTimelock {
  isActive: boolean;
  timeEnforcementEnds: number; // unix seconds (0 when not active)
  enforcementType: string;
}

export interface NewChatCapping {
  cappingStatus: string;
  totalQuota: number;
  usedQuota: number;
  cycleEnds: number; // unix seconds
}

export interface InstanceLimits {
  reachoutTimelock: ReachoutTimelock | null;
  newChatCapping: NewChatCapping | null;
}

/**
 * Get WhatsApp account-level messaging limits for an instance
 * GET /instance/limits/:instanceId
 * Returns the reachout timelock (cause of error 463) and new-chat quota.
 */
export const getInstanceLimits = async (
  instanceId: string
): Promise<InstanceLimits> => {
  const response = await apiClient.get<{
    message: string;
    data: InstanceLimits;
  }>(`/instance/limits/${instanceId}`);
  return response.data.data;
};

/**
 * Create a new instance
 * POST /instance/create
 */
export const createInstance = async (
  payload: CreateInstancePayload
): Promise<Instance> => {
  const response = await apiClient.post<Instance>('/instance/create', payload);
  return response.data;
};

export interface ConnectConfig {
  webhookUrl?: string;
  subscribe?: string[];
  phone?: string;
  rabbitmqEnable?: string;
  websocketEnable?: string;
  natsEnable?: string;
  alwaysOnline?: boolean;
  rejectCall?: boolean;
  readMessages?: boolean;
  ignoreGroups?: boolean;
  ignoreStatus?: boolean;
}

export interface PairConfig {
  subscribe: string[];
  phone: string;
}

export interface AdvancedSettings {
  alwaysOnline?: boolean;
  rejectCall?: boolean;
  readMessages?: boolean;
  ignoreGroups?: boolean;
  ignoreStatus?: boolean;
}

/**
 * Connect to an instance (get QR code or connection status)
 * POST /instance/connect
 * Requires instance token in apikey header
 */
export const connectInstance = async (
  instanceToken: string,
  config?: ConnectConfig
): Promise<{ jid: string; webhookUrl: string; eventString: string }> => {
  const payload = {
    webhookUrl: config?.webhookUrl || '',
    subscribe: config?.subscribe || [],
    rabbitmqEnable: config?.rabbitmqEnable || '',
    websocketEnable: config?.websocketEnable || '',
    natsEnable: config?.natsEnable || '',
  };

  const response = await apiClient.post<{
    message: string;
    data: { jid: string; webhookUrl: string; eventString: string };
  }>(
    '/instance/connect',
    payload,
    {
      headers: {
        apikey: instanceToken,
      },
    }
  );
  return response.data.data;
};

/**
 * Pair instance with phone number (get pairing code)
 * POST /instance/pair
 * Requires instance token in apikey header
 */
export const pairInstance = async (
  instanceToken: string,
  config: PairConfig
): Promise<{ pairingCode: string }> => {
  const response = await apiClient.post<{
    message: string;
    // The Go struct field has no json tag, so it serializes as "PairingCode".
    // camelCase is accepted too in case the tag is added upstream.
    data: { PairingCode?: string; pairingCode?: string };
  }>(
    '/instance/pair',
    {
      subscribe: config.subscribe,
      phone: config.phone,
    },
    {
      headers: {
        apikey: instanceToken,
      },
    }
  );

  const data = response.data?.data;
  const pairingCode = data?.PairingCode ?? data?.pairingCode ?? '';

  if (!pairingCode) {
    throw new Error(
      'A API não retornou um código de pareamento. Verifique se o número está correto e se a instância não está já conectada.'
    );
  }

  return { pairingCode };
};

/**
 * Get advanced settings for an instance
 * GET /instance/:instanceId/advanced-settings
 * Requires instance token in apikey header
 */
export const getAdvancedSettings = async (
  instanceId: string,
  instanceToken: string
): Promise<AdvancedSettings> => {
  const response = await apiClient.get<{
    message: string;
    data: AdvancedSettings;
  }>(
    `/instance/${instanceId}/advanced-settings`,
    {
      headers: {
        apikey: instanceToken,
      },
    }
  );
  return response.data.data;
};

/**
 * Update advanced settings for an instance
 * PUT /instance/:instanceId/advanced-settings
 * Requires instance token in apikey header
 */
export const updateAdvancedSettings = async (
  instanceId: string,
  instanceToken: string,
  settings: AdvancedSettings
): Promise<void> => {
  await apiClient.put(
    `/instance/${instanceId}/advanced-settings`,
    settings,
    {
      headers: {
        apikey: instanceToken,
      },
    }
  );
};

export interface QrResult {
  qrcode: string;
  code: string;
  // Set when the account requires a WebAuthn passkey to finish linking (no QR).
  passkeyStage?: string;
  passkeyOpenUrl?: string;
  passkeyCode?: string;
}

/**
 * Get QR Code (or passkey ceremony state) for an instance
 * GET /instance/qr
 * Requires instance token in apikey header
 */
export const getQrCode = async (
  instanceToken: string
): Promise<QrResult> => {
  const response = await apiClient.get<{
    message: string;
    data: {
      Qrcode?: string;
      Code?: string;
      passkeyStage?: string;
      passkeyOpenUrl?: string;
      passkeyCode?: string;
    };
  }>('/instance/qr', {
    headers: {
      apikey: instanceToken,
    },
  });

  const d = response.data.data;
  return {
    qrcode: d.Qrcode ?? '',
    code: d.Code ?? '',
    passkeyStage: d.passkeyStage,
    passkeyOpenUrl: d.passkeyOpenUrl,
    passkeyCode: d.passkeyCode,
  };
};

/**
 * Get connection status of an instance
 * GET /instance/status
 */
export const getConnectionState = async (): Promise<ConnectionState> => {
  const response = await apiClient.get<ConnectionState>('/instance/status');
  return response.data;
};

/**
 * Disconnect/logout an instance
 * DELETE /instance/logout
 */
export const logoutInstance = async (instanceToken: string): Promise<void> => {
  await apiClient.delete('/instance/logout', {
    headers: {
      apikey: instanceToken,
    },
  });
};

/**
 * Delete an instance permanently
 * DELETE /instance/delete/:instanceId
 */
export const deleteInstance = async (instanceId: string): Promise<void> => {
  await apiClient.delete(`/instance/delete/${instanceId}`);
};

/**
 * Reconnect an instance
 * POST /instance/reconnect
 * Requires instance token in apikey header — the route is scoped to the
 * instance that owns the token, not to the admin key.
 */
export const restartInstance = async (instanceToken: string): Promise<void> => {
  await apiClient.post('/instance/reconnect', undefined, {
    headers: {
      apikey: instanceToken,
    },
  });
};

export interface ProxyConfig {
  host: string;
  port: string;
  username?: string;
  password?: string;
  protocol?: string;
}

/**
 * Set or update proxy configuration for an instance
 * POST /instance/proxy/:instanceId
 * Requires admin apikey header
 */
export const setInstanceProxy = async (
  instanceId: string,
  proxy: ProxyConfig
): Promise<void> => {
  await apiClient.post(`/instance/proxy/${instanceId}`, proxy);
};

/**
 * Get the proxy configuration saved for an instance
 * GET /instance/proxy/:instanceId
 * Requires admin apikey header. Returns null when no proxy is configured.
 */
export const getInstanceProxy = async (
  instanceId: string
): Promise<ProxyConfig | null> => {
  const response = await apiClient.get<{
    message: string;
    data: ProxyConfig | null;
  }>(`/instance/proxy/${instanceId}`);
  return response.data.data ?? null;
};

/**
 * Reconnect an instance through its already-saved proxy
 * POST /instance/proxy/:instanceId/reconnect
 * Requires admin apikey header
 */
export const reconnectInstanceProxy = async (
  instanceId: string
): Promise<void> => {
  await apiClient.post(`/instance/proxy/${instanceId}/reconnect`);
};

/**
 * Result of probing a proxy: whether traffic gets through and which IP it
 * exits from. `anonymous` is false when the exit IP matches the server's own,
 * which means the proxy is not actually masking anything.
 */
export interface ProxyTestResult {
  ok: boolean;
  ip?: string;
  serverIp?: string;
  anonymous: boolean;
  whatsappReachable: boolean;
  latencyMs?: number;
  protocol?: string;
  error?: string;
}

/**
 * Test a proxy without touching the instance's live connection.
 * POST /instance/proxy/:instanceId/test
 * Pass a proxy to check one before saving it; omit it to test the saved one.
 * Requires admin apikey header
 */
export const testInstanceProxy = async (
  instanceId: string,
  proxy?: ProxyConfig
): Promise<ProxyTestResult> => {
  const response = await apiClient.post<ProxyTestResult>(
    `/instance/proxy/${instanceId}/test`,
    proxy ?? {}
  );
  return response.data;
};

/**
 * Remove proxy configuration from an instance
 * DELETE /instance/proxy/:instanceId
 * Requires admin apikey header
 */
export const removeInstanceProxy = async (instanceId: string): Promise<void> => {
  await apiClient.delete(`/instance/proxy/${instanceId}`);
};

/**
 * Send a text message
 * POST /send/text
 * Requires instance token in apikey header
 */
export const sendMessage = async (
  instanceToken: string,
  payload: { number: string; text: string }
): Promise<{ message: string; data: unknown }> => {
  const response = await apiClient.post<{
    message: string;
    data: unknown;
  }>(
    '/send/text',
    payload,
    {
      headers: {
        apikey: instanceToken,
      },
    }
  );
  return response.data;
};

/**
 * Send a button message (test scenarios)
 * POST /send/button
 */
export const sendButtonMessage = async (
  instanceToken: string,
  payload: Record<string, unknown>
): Promise<{ message: string; data: unknown }> => {
  const response = await apiClient.post<{ message: string; data: unknown }>(
    '/send/button',
    payload,
    { headers: { apikey: instanceToken } }
  );
  return response.data;
};

/**
 * Send a list message (test scenarios)
 * POST /send/list
 */
export const sendListMessage = async (
  instanceToken: string,
  payload: Record<string, unknown>
): Promise<{ message: string; data: unknown }> => {
  const response = await apiClient.post<{ message: string; data: unknown }>(
    '/send/list',
    payload,
    { headers: { apikey: instanceToken } }
  );
  return response.data;
};

/**
 * Send a carousel message (test scenarios)
 * POST /send/carousel
 */
export const sendCarouselMessage = async (
  instanceToken: string,
  payload: Record<string, unknown>
): Promise<{ message: string; data: unknown }> => {
  const response = await apiClient.post<{ message: string; data: unknown }>(
    '/send/carousel',
    payload,
    { headers: { apikey: instanceToken } }
  );
  return response.data;
};

export interface CallSnapshot {
  callId: string;
  instanceId: string;
  peerJid: string;
  number: string;
  direction: 'outgoing' | 'incoming';
  mediaType: string;
  state: string;
  endReason?: string;
  createdAt: string;
  acceptedAt?: string;
  endedAt?: string;
  durationSeconds: number;
}

/**
 * Place a WhatsApp call
 * POST /call/offer
 */
export const offerCall = async (
  instanceToken: string,
  payload: { number: string; video?: boolean }
): Promise<CallSnapshot> => {
  const response = await apiClient.post<{ message: string; data: CallSnapshot }>(
    '/call/offer',
    payload,
    { headers: { apikey: instanceToken } }
  );
  return response.data.data;
};

/**
 * Hang up a call
 * POST /call/terminate
 */
export const terminateCall = async (
  instanceToken: string,
  callId: string,
  reason?: string
): Promise<void> => {
  await apiClient.post(
    '/call/terminate',
    { callId, reason },
    { headers: { apikey: instanceToken } }
  );
};

/**
 * List the calls currently ringing or active
 * GET /call/list
 */
export const listCalls = async (
  instanceToken: string
): Promise<CallSnapshot[]> => {
  const response = await apiClient.get<{ message: string; data: CallSnapshot[] }>(
    '/call/list',
    { headers: { apikey: instanceToken } }
  );
  return response.data.data ?? [];
};

/**
 * Read one call by id
 * GET /call/status/:callId
 */
export const getCallStatus = async (
  instanceToken: string,
  callId: string
): Promise<CallSnapshot> => {
  const response = await apiClient.get<{ message: string; data: CallSnapshot }>(
    `/call/status/${callId}`,
    { headers: { apikey: instanceToken } }
  );
  return response.data.data;
};

export default {
  fetchInstances,
  offerCall,
  terminateCall,
  listCalls,
  getCallStatus,
  fetchInstance,
  getInstanceLimits,
  createInstance,
  connectInstance,
  pairInstance,
  getAdvancedSettings,
  updateAdvancedSettings,
  getQrCode,
  getConnectionState,
  logoutInstance,
  deleteInstance,
  restartInstance,
  getInstanceProxy,
  reconnectInstanceProxy,
  testInstanceProxy,
  setInstanceProxy,
  removeInstanceProxy,
  sendMessage,
  sendButtonMessage,
  sendListMessage,
  sendCarouselMessage,
};
