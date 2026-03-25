/**
 * Instance Types for Evolution GO Manager
 * Based on Evolution GO API responses
 */

export type InstanceStatus = 'open' | 'close';

// Raw instance from Evolution GO API
export interface RawInstance {
  id: string;
  name: string;
  token: string;
  webhook: string;
  rabbitmqEnable: string;
  websocketEnable: string;
  natsEnable: string;
  jid: string;
  qrcode: string;
  connected: boolean;
  expiration: number;
  disconnect_reason: string;
  events: string;
  os_name: string;
  proxy: string;
  client_name: string;
  createdAt: string;
  alwaysOnline: boolean;
  rejectCall: boolean;
  msgRejectCall: string;
  readMessages: boolean;
  ignoreGroups: boolean;
  ignoreStatus: boolean;
}

// Normalized instance for our app
export interface Instance {
  id: string;
  instanceName: string;
  status: InstanceStatus;
  serverUrl?: string;
  apikey?: string;
  owner: string;
  profileName?: string;
  profilePicUrl?: string;
  profileStatus?: string;
  qrcode?: {
    base64?: string;
    code?: string;
    pairingCode?: string;
  };
  integration?: string;
  webhook?: string;
  rabbitmqEnable?: string;
  websocketEnable?: string;
  natsEnable?: string;
  events?: string;
  connected: boolean;
  disconnectReason?: string;
  createdAt?: string;
  updatedAt?: string;
  alwaysOnline?: boolean;
  rejectCall?: boolean;
  readMessages?: boolean;
  ignoreGroups?: boolean;
  ignoreStatus?: boolean;
}

export interface CreateInstancePayload {
  instanceId?: string;
  name: string;
  token: string;
  proxy?: {
    host: string;
    port: string;
    username?: string;
    password?: string;
  };
  advancedSettings?: {
    alwaysOnline?: boolean;
    rejectCall?: boolean;
    msgRejectCall?: string;
    readMessages?: boolean;
    ignoreGroups?: boolean;
    ignoreStatus?: boolean;
  };
}

export interface ConnectionState {
  instance: string;
  state: InstanceStatus;
}

export interface InstancesResponse {
  data: RawInstance[];
  message: string;
}
