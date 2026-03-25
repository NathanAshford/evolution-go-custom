import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios';

/**
 * API Client configuration
 *
 * This client is configured to work with Evolution GO API
 * It automatically adds the apikey header and handles authentication
 */

// Create axios instance
const apiClient: AxiosInstance = axios.create({
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor - Add baseURL and admin apikey when needed
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Get auth data from localStorage (Zustand persist format)
    const authData = localStorage.getItem('evolution-auth');

    if (authData) {
      try {
        const parsed = JSON.parse(authData);
        // Zustand persist wraps data in { state: { ... } }
        const { apiUrl, apiKey } = parsed.state || parsed;

        // Only set baseURL if not explicitly provided in the request config
        if (apiUrl && !config.baseURL) {
          config.baseURL = apiUrl;
        }

        // Only add admin apikey if NO apikey header was set
        // Instance-specific operations will set their own instance token
        if (apiKey && !config.headers.has('apikey')) {
          config.headers.set('apikey', apiKey);
        }
      } catch (error) {
        console.error('Error parsing auth data:', error);
      }
    }

    return config;
  },
  (error: AxiosError) => {
    return Promise.reject(error);
  }
);

// Response interceptor - Handle errors and authentication
apiClient.interceptors.response.use(
  (response) => {
    return response;
  },
  (error: AxiosError) => {
    // Handle 401 Unauthorized - logout user and reset license
    if (error.response?.status === 401) {
      console.error('Unauthorized - clearing auth data');
      localStorage.removeItem('evolution-auth');
      window.location.href = '/manager/login';
      return Promise.reject(error);
    }

    // Handle 403 Forbidden (license invalid) - reset license state and redirect
    if (error.response?.status === 403) {
      const data = error.response?.data as { code?: string } | undefined;
      if (data?.code === 'LICENSE_INVALID' || data?.code === 'LICENSE_EXPIRED') {
        console.error('License invalid - redirecting to login');
        const authData = localStorage.getItem('evolution-auth');
        if (authData) {
          try {
            const parsed = JSON.parse(authData);
            const state = parsed.state || parsed;
            state.licenseState = 'unlicensed';
            if (parsed.state) parsed.state = state;
            localStorage.setItem('evolution-auth', JSON.stringify(parsed));
          } catch { /* ignore */ }
        }
        window.location.href = '/manager/login';
        return Promise.reject(error);
      }
    }

    // Handle network errors
    if (!error.response) {
      console.error('Network error:', error.message);
      return Promise.reject({
        message: 'Erro de conexão. Verifique sua internet e tente novamente.',
        originalError: error,
      });
    }

    // Handle API errors
    const errorMessage = error.response?.data
      ? (error.response.data as { error?: string; message?: string }).error ||
        (error.response.data as { error?: string; message?: string }).message ||
        'Erro desconhecido'
      : error.message;

    console.error('API Error:', errorMessage);

    return Promise.reject({
      message: errorMessage,
      status: error.response?.status,
      originalError: error,
    });
  }
);

export default apiClient;
