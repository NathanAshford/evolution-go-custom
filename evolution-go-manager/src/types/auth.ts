/**
 * Authentication types
 */

export type LicenseState = 'unchecked' | 'licensed' | 'unlicensed' | 'pending';

export interface AuthState {
  apiUrl: string;
  apiKey: string;
  isAuthenticated: boolean;
  licenseState: LicenseState;
}

export interface AuthStore extends AuthState {
  login: (apiUrl: string, apiKey: string) => Promise<void>;
  logout: () => void;
  setApiUrl: (apiUrl: string) => void;
  setApiKey: (apiKey: string) => void;
  setLicenseState: (state: LicenseState) => void;
  checkLicense: (apiUrl?: string, apiKey?: string) => Promise<LicenseState>;
}

export interface LoginCredentials {
  apiUrl: string;
  apiKey: string;
}

export interface LoginResponse {
  success: boolean;
  message?: string;
}
