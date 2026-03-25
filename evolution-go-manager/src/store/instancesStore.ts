/**
 * Instances Store
 * Manages instances state using Zustand
 */

import { create } from 'zustand';
import type { Instance } from '@/types/instance';
import * as instancesApi from '@/services/api/instances';

interface InstancesStore {
  instances: Instance[];
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchInstances: () => Promise<void>;
  addInstance: (instance: Instance) => void;
  updateInstance: (instanceName: string, updates: Partial<Instance>) => void;
  removeInstance: (instanceName: string) => void;
  setLoading: (isLoading: boolean) => void;
  setError: (error: string | null) => void;
  clearError: () => void;
}

const useInstancesStore = create<InstancesStore>()((set) => ({
  instances: [],
  isLoading: false,
  error: null,

  // Fetch all instances from API
  fetchInstances: async () => {
    set({ isLoading: true, error: null });
    try {
      const instances = await instancesApi.fetchInstances();
      set({ instances, isLoading: false });
    } catch (error) {
      console.error('Failed to fetch instances:', error);
      set({
        error:
          error instanceof Error
            ? error.message
            : 'Erro ao buscar instâncias',
        isLoading: false,
      });
    }
  },

  // Add a new instance to the store
  addInstance: (instance: Instance) => {
    set((state) => ({
      instances: [...state.instances, instance],
    }));
  },

  // Update an existing instance
  updateInstance: (instanceName: string, updates: Partial<Instance>) => {
    set((state) => ({
      instances: state.instances.map((instance) =>
        instance.instanceName === instanceName
          ? { ...instance, ...updates }
          : instance
      ),
    }));
  },

  // Remove an instance from the store
  removeInstance: (instanceName: string) => {
    set((state) => ({
      instances: state.instances.filter(
        (instance) => instance.instanceName !== instanceName
      ),
    }));
  },

  // Set loading state
  setLoading: (isLoading: boolean) => {
    set({ isLoading });
  },

  // Set error message
  setError: (error: string | null) => {
    set({ error });
  },

  // Clear error
  clearError: () => {
    set({ error: null });
  },
}));

export default useInstancesStore;
