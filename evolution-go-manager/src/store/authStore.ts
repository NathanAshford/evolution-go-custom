import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import apiClient from '@/services/api/client';
import { checkLicenseStatus } from '@/services/api/license';
import type { AuthStore, LicenseState } from '@/types/auth';

/**
 * Authentication Store
 *
 * Manages authentication state including API URL and API Key
 * Persists data to localStorage
 */

const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      // Initial state
      apiUrl: typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8082',
      apiKey: '',
      isAuthenticated: false,
      licenseState: 'unchecked' as LicenseState,

      // Login method - validates connection and stores credentials
      login: async (apiUrl: string, apiKey: string) => {
        // Remove trailing slash from URL
        const cleanUrl = apiUrl.replace(/\/$/, '');

        try {
          // Validate apikey against an admin-protected endpoint.
          // /server/ok is public and does NOT authenticate the apikey,
          // so hitting it would accept any value. /instance/all requires
          // AuthAdmin (apikey == GLOBAL_API_KEY), which is what we need.
          await apiClient.get('/instance/all', {
            baseURL: cleanUrl,
            headers: {
              apikey: apiKey,
              'Cache-Control': 'no-cache',
            },
            params: { t: Date.now() },
          });

          // If request succeeded (2xx), credentials are valid
          set({
            apiUrl: cleanUrl,
            apiKey,
            isAuthenticated: true,
          });
        } catch (error: unknown) {
          console.error('Login error:', error);
          const status = (error as { response?: { status?: number } })?.response?.status;

          // Make sure we do NOT mark the session as authenticated on failure.
          set({ isAuthenticated: false });

          if (status === 401 || status === 403) {
            throw new Error('API Key invalida. Verifique a chave informada.');
          }
          throw new Error(
            'Nao foi possivel conectar. Verifique a URL e a API Key.'
          );
        }
      },

      // Logout method - clears credentials and localStorage
      logout: () => {
        // Clear localStorage
        localStorage.removeItem('evolution-auth');
        
        // Reset state (including license)
        set({
          apiUrl: typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8082',
          apiKey: '',
          isAuthenticated: false,
          licenseState: 'unchecked' as LicenseState,
        });
        
        // Redirect to login page
        window.location.href = '/manager/login';
      },

      // Update API URL
      setApiUrl: (apiUrl: string) => {
        set({ apiUrl: apiUrl.replace(/\/$/, '') });
      },

      // Update API Key
      setApiKey: (apiKey: string) => {
        set({ apiKey });
      },

      // Update license state
      setLicenseState: (licenseState: LicenseState) => {
        set({ licenseState });
      },

      // Check license status against backend
      checkLicense: async (apiUrl?: string, apiKey?: string) => {
        try {
          const result = await checkLicenseStatus(apiUrl, apiKey);
          const state: LicenseState = result.status === 'active' ? 'licensed' : 'unlicensed';
          set({ licenseState: state });
          return state;
        } catch (error) {
          console.error('License check error:', error);
          set({ licenseState: 'unlicensed' });
          return 'unlicensed' as LicenseState;
        }
      },
    }),
    {
      name: 'evolution-auth', // localStorage key
      partialize: (state) => ({
        apiUrl: state.apiUrl,
        apiKey: state.apiKey,
        isAuthenticated: state.isAuthenticated,
        licenseState: state.licenseState,
      }),
    }
  )
);

export default useAuthStore;
