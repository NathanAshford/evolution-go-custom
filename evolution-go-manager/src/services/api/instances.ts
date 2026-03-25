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
} from '@/types/instance';

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
    data: { PairingCode: string };
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

  return {
    pairingCode: response.data.data.PairingCode,
  };
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

/**
 * Get QR Code for an instance
 * GET /instance/qr
 * Requires instance token in apikey header
 */
export const getQrCode = async (
  instanceToken: string
): Promise<{ qrcode: string; code: string }> => {
  const response = await apiClient.get<{
    message: string;
    data: { Qrcode: string; Code: string };
  }>('/instance/qr', {
    headers: {
      apikey: instanceToken,
    },
  });

  // Map to lowercase for consistency
  return {
    qrcode: response.data.data.Qrcode,
    code: response.data.data.Code,
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
 */
export const restartInstance = async (): Promise<void> => {
  await apiClient.post('/instance/reconnect');
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

export default {
  fetchInstances,
  fetchInstance,
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
  sendMessage,
};
