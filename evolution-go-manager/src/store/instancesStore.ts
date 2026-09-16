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
  /**
   * True once a fetch has completed at least one time (successfully or not).
   * Used to distinguish the very first load — which should render skeletons —
   * from background polling refreshes, which must keep the current list on
   * screen so the grid does not flash/teardown every few seconds.
   */
  hasLoaded: boolean;
  /** True while a background refresh (polling) is in flight. */
  isRefreshing: boolean;
  error: string | null;

  // Actions
  fetchInstances: (options?: { silent?: boolean }) => Promise<void>;
  addInstance: (instance: Instance) => void;
  updateInstance: (instanceName: string, updates: Partial<Instance>) => void;
  removeInstance: (instanceName: string) => void;
  setLoading: (isLoading: boolean) => void;
  setError: (error: string | null) => void;
  clearError: () => void;
}

const useInstancesStore = create<InstancesStore>()((set, get) => ({
  instances: [],
  isLoading: false,
  hasLoaded: false,
  isRefreshing: false,
  error: null,

  // Fetch all instances from API.
  // `silent` (used by polling) never raises the blocking `isLoading` flag, so
  // the already-rendered list stays mounted while data refreshes underneath.
  fetchInstances: async (options) => {
    const silent = options?.silent ?? false;
    const { hasLoaded } = get();
    const blocking = !silent && !hasLoaded;

    set(
      blocking
        ? { isLoading: true, isRefreshing: true, error: null }
        : { isRefreshing: true }
    );

    try {
      const instances = await instancesApi.fetchInstances();
      set({
        instances,
        isLoading: false,
        isRefreshing: false,
        hasLoaded: true,
        error: null,
      });
    } catch (error) {
      console.error('Failed to fetch instances:', error);
      const message =
        error instanceof Error
          ? error.message
          : (error as { message?: string })?.message ||
            'Erro ao buscar instâncias';
      set({
        error: message,
        isLoading: false,
        isRefreshing: false,
        hasLoaded: true,
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
