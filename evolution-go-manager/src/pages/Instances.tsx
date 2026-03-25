import { useEffect, useMemo, useState, useCallback, useRef } from 'react';
import {
  Button,
  Skeleton,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
} from '@evoapi/design-system';
import { Trash2, Layers } from 'lucide-react';
import { toast } from 'sonner';

import useInstancesStore from '@/store/instancesStore';
import { InstanceCard, InstancesHeader, CreateInstanceModal, QRCodeModal, ConnectConfigModal } from '@/components/instances';
import SendMessageModal from '@/components/instances/SendMessageModal';
import EmptyState from '@/components/base/EmptyState';
import type { Instance } from '@/types/instance';
import * as instancesApi from '@/services/api/instances';
import type { ConnectConfig } from '@/services/api/instances';
import { useNavigate } from 'react-router-dom';

export default function Instances() {
  const navigate = useNavigate();
  const { instances, isLoading, fetchInstances, removeInstance } =
    useInstancesStore();
  const [query, setQuery] = useState('');
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [qrcodeModal, setQrcodeModal] = useState<{
    isOpen: boolean;
    instance: Instance | null;
  }>({
    isOpen: false,
    instance: null,
  });

  // Pagination states
  const [currentPage] = useState(1);
  const [perPage] = useState(24);
  const [isDeleting, setIsDeleting] = useState<string | null>(null);
  const [deleteModal, setDeleteModal] = useState<{
    isOpen: boolean;
    instance: Instance | null;
    confirmationText: string;
  }>({
    isOpen: false,
    instance: null,
    confirmationText: '',
  });
  const [sendMessageModal, setSendMessageModal] = useState<{
    isOpen: boolean;
    instance: Instance | null;
  }>({
    isOpen: false,
    instance: null,
  });
  const [connectConfigModal, setConnectConfigModal] = useState<{
    isOpen: boolean;
    instance: Instance | null;
  }>({
    isOpen: false,
    instance: null,
  });

  // Ref to track if initial fetch was done
  const initialFetchDone = useRef(false);

  useEffect(() => {
    // Only fetch once on mount
    if (!initialFetchDone.current) {
      fetchInstances();
      initialFetchDone.current = true;
    }

    // Polling: atualizar instâncias a cada 5 segundos
    const interval = setInterval(() => {
      fetchInstances();
    }, 5000);

    return () => clearInterval(interval);
  }, [fetchInstances]);

  const { paginatedInstances, totalCount } = useMemo(() => {
    // First filter by search query
    const filtered = query
      ? instances.filter((i: Instance) => {
          const q = query.toLowerCase();
          return (
            i.instanceName?.toLowerCase().includes(q) ||
            i.profileName?.toLowerCase().includes(q) ||
            i.owner?.toLowerCase().includes(q)
          );
        })
      : instances;

    // Then paginate
    const startIndex = (currentPage - 1) * perPage;
    const endIndex = startIndex + perPage;
    const paginated = filtered.slice(startIndex, endIndex);

    return {
      paginatedInstances: paginated,
      totalCount: filtered.length,
    };
  }, [instances, query, currentPage, perPage]);

  const handleNewInstance = useCallback(() => {
    setCreateModalOpen(true);
  }, []);

  const handleSettings = useCallback(
    (instance: Instance) => {
      navigate(`/manager/instances/${instance.id}/settings`);
    },
    [navigate]
  );

  const handleConnect = useCallback((instance: Instance) => {
    // Open config modal before connecting
    setConnectConfigModal({
      isOpen: true,
      instance,
    });
  }, []);

  const handleConnectWithConfig = useCallback(async (instance: Instance, config: ConnectConfig) => {
    try {
      if (!instance.apikey) {
        toast.error('Token da instância não encontrado');
        return;
      }

      toast.info(`Conectando ${instance.instanceName}...`);

      // Step 1: Update advanced settings if any are provided
      const hasAdvancedSettings =
        config.alwaysOnline !== undefined ||
        config.rejectCall !== undefined ||
        config.readMessages !== undefined ||
        config.ignoreGroups !== undefined ||
        config.ignoreStatus !== undefined;

      if (hasAdvancedSettings) {
        const advancedSettings = {
          alwaysOnline: config.alwaysOnline,
          rejectCall: config.rejectCall,
          readMessages: config.readMessages,
          ignoreGroups: config.ignoreGroups,
          ignoreStatus: config.ignoreStatus,
        };
        await instancesApi.updateAdvancedSettings(instance.id, instance.apikey, advancedSettings);
      }

      // Check if phone is provided for pairing
      const usesPairingCode = config.phone && config.phone.trim() !== '';

      if (usesPairingCode) {
        // Use pairing code endpoint
        let phone = config.phone!.trim();

        // Add + prefix if not present
        if (!phone.startsWith('+')) {
          phone = `+${phone}`;
        }

        // Step 2: Pair instance with phone number
        await instancesApi.connectInstance(instance.apikey, {
          ...config,
          phone: undefined, // Don't send phone to connect endpoint
        });

        // Wait a bit for connection to initialize
        await new Promise(resolve => setTimeout(resolve, 1000));

        // Step 3: Get pairing code
        const pairData = await instancesApi.pairInstance(instance.apikey, {
          subscribe: config.subscribe || [],
          phone,
        });

        // Step 4: Get QR code as well (both QR and pairing code)
        try {
          const qrData = await instancesApi.getQrCode(instance.apikey);

          // Update instance with both QR code and pairing code
          const instanceWithQr: Instance = {
            ...instance,
            qrcode: {
              base64: qrData.qrcode,
              code: qrData.code,
              pairingCode: pairData.pairingCode,
            },
          };

          // Open QR Code modal (will show both QR and pairing code)
          setQrcodeModal({
            isOpen: true,
            instance: instanceWithQr,
          });
          toast.success(`QR Code e Pairing Code gerados para ${instance.instanceName}!`);
        } catch {
          // If QR code fails, still show pairing code
          const instanceWithPair: Instance = {
            ...instance,
            qrcode: {
              pairingCode: pairData.pairingCode,
            },
          };

          setQrcodeModal({
            isOpen: true,
            instance: instanceWithPair,
          });
          toast.success(`Pairing Code gerado para ${instance.instanceName}!`);
        }
      } else {
        // Step 2: Connect instance with webhook config (QR code only)
        await instancesApi.connectInstance(instance.apikey, config);

        // Wait a bit for QR code generation
        await new Promise(resolve => setTimeout(resolve, 2000));

        // Get QR Code from API
        try {
          const qrData = await instancesApi.getQrCode(instance.apikey);

          // Update instance with QR code data
          const instanceWithQr: Instance = {
            ...instance,
            qrcode: {
              base64: qrData.qrcode,
              code: qrData.code,
            },
          };

          // Open QR Code modal
          setQrcodeModal({
            isOpen: true,
            instance: instanceWithQr,
          });
          toast.success(`QR Code gerado para ${instance.instanceName}!`);
        } catch (qrError) {
          console.error('Erro ao buscar QR Code:', qrError);
          toast.error('QR Code ainda não disponível, aguarde alguns segundos...');

          // Fallback: fetch all instances
          await fetchInstances();
          const updatedInstance = instances.find(
            (i) => i.instanceName === instance.instanceName
          );
          if (updatedInstance) {
            setQrcodeModal({
              isOpen: true,
              instance: updatedInstance,
            });
          }
        }
      }
    } catch (error) {
      console.error('Erro ao conectar instância:', error);
      toast.error(
        error instanceof Error
          ? error.message
          : 'Erro ao conectar instância'
      );
    }
  }, [fetchInstances, instances]);

  const handleDisconnect = useCallback(async (instance: Instance) => {
    try {
      if (!instance.apikey) {
        toast.error('Token da instância não encontrado');
        return;
      }
      toast.info(`Desconectando ${instance.instanceName}...`);
      await instancesApi.logoutInstance(instance.apikey);
      await fetchInstances();
      toast.success(`${instance.instanceName} desconectada!`);
    } catch (error) {
      console.error('Erro ao desconectar instância:', error);
      toast.error(
        error instanceof Error
          ? error.message
          : 'Erro ao desconectar instância'
      );
    }
  }, [fetchInstances]);

  const openDeleteModal = (instance: Instance) => {
    setDeleteModal({
      isOpen: true,
      instance,
      confirmationText: '',
    });
  };

  const closeDeleteModal = () => {
    setDeleteModal({
      isOpen: false,
      instance: null,
      confirmationText: '',
    });
  };

  const closeQrcodeModal = () => {
    setQrcodeModal({
      isOpen: false,
      instance: null,
    });
  };

  const openSendMessageModal = (instance: Instance) => {
    setSendMessageModal({
      isOpen: true,
      instance,
    });
  };

  const closeSendMessageModal = () => {
    setSendMessageModal({
      isOpen: false,
      instance: null,
    });
  };

  const closeConnectConfigModal = () => {
    setConnectConfigModal({
      isOpen: false,
      instance: null,
    });
  };

  const handleConnectConfirm = useCallback((config: ConnectConfig) => {
    if (connectConfigModal.instance) {
      handleConnectWithConfig(connectConfigModal.instance, config);
    }
  }, [connectConfigModal.instance, handleConnectWithConfig]);

  const handleRefreshQrcode = useCallback(async () => {
    if (!qrcodeModal.instance || !qrcodeModal.instance.apikey) return;

    try {
      // Fetch fresh instance data to check connection status
      await fetchInstances();

      // Find updated instance
      const updatedInstance = instances.find(
        (i) => i.id === qrcodeModal.instance?.id
      );

      if (!updatedInstance) return;

      // If connected, just update modal and it will show success screen
      if (updatedInstance.connected) {
        setQrcodeModal({
          isOpen: true,
          instance: updatedInstance,
        });
        return;
      }

      // If not connected, get fresh QR Code
      try {
        const qrData = await instancesApi.getQrCode(qrcodeModal.instance.apikey);

        // Update the modal with fresh QR code
        const instanceWithQr: Instance = {
          ...updatedInstance,
          qrcode: {
            base64: qrData.qrcode,
            code: qrData.code,
          },
        };

        setQrcodeModal({
          isOpen: true,
          instance: instanceWithQr,
        });
      } catch {
        // QR might be expired, just update with fetched data
        setQrcodeModal({
          isOpen: true,
          instance: updatedInstance,
        });
      }
    } catch (error) {
      console.error('Erro ao atualizar QR Code:', error);
      throw error;
    }
  }, [qrcodeModal.instance, fetchInstances, instances]);

  const handleConfirmDelete = async () => {
    if (!deleteModal.instance) return;

    const instanceName = deleteModal.instance.instanceName;
    const instanceId = deleteModal.instance.id;
    setIsDeleting(instanceName);

    try {
      await instancesApi.deleteInstance(instanceId);

      // Optimistically remove from local state
      removeInstance(instanceName);

      toast.success(`Instância ${instanceName} removida com sucesso!`);
      closeDeleteModal();
    } catch (e: unknown) {
      console.error('Erro ao remover instância:', e);
      toast.error(
        (e as Error)?.message || 'Erro ao remover instância'
      );

      // Refresh list on error to restore correct state
      await fetchInstances();
    } finally {
      setIsDeleting(null);
    }
  };

  const isDeleteConfirmationValid =
    deleteModal.confirmationText === deleteModal.instance?.instanceName;

  return (
    <div className="h-full flex flex-col p-4">
      <InstancesHeader
        totalCount={totalCount}
        selectedCount={0}
        searchValue={query}
        onSearchChange={setQuery}
        onNewInstance={handleNewInstance}
        onClearSelection={() => {}}
      />

      {/* View Mode Toggle (removed for now, only cards view) */}

      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, idx) => (
              <Skeleton key={idx} className="h-48" />
            ))}
          </div>
        ) : totalCount === 0 ? (
          <EmptyState
            icon={Layers}
            title="Nenhuma instância encontrada"
            description="Crie sua primeira instância para começar a usar o Evolution GO"
            action={{ label: 'Nova Instância', onClick: handleNewInstance }}
            className="h-full"
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
            {paginatedInstances.map((instance: Instance) => (
              <InstanceCard
                key={instance.instanceName}
                instance={instance}
                isDeleting={isDeleting}
                onSettings={handleSettings}
                onDelete={openDeleteModal}
                onConnect={handleConnect}
                onDisconnect={handleDisconnect}
                onSendMessage={openSendMessageModal}
              />
            ))}
          </div>
        )}
      </div>

      {/* Create Instance Modal */}
      <CreateInstanceModal
        open={createModalOpen}
        onOpenChange={setCreateModalOpen}
      />

      {/* QR Code Modal */}
      <QRCodeModal
        instance={qrcodeModal.instance}
        open={qrcodeModal.isOpen}
        onOpenChange={closeQrcodeModal}
        onRefresh={handleRefreshQrcode}
      />

      {/* Connect Config Modal */}
      <ConnectConfigModal
        instance={connectConfigModal.instance}
        open={connectConfigModal.isOpen}
        onClose={closeConnectConfigModal}
        onConfirm={handleConnectConfirm}
      />

      {/* Send Message Modal */}
      <SendMessageModal
        instance={sendMessageModal.instance}
        open={sendMessageModal.isOpen}
        onClose={closeSendMessageModal}
      />

      {/* Delete Confirmation Modal */}
      <Dialog open={deleteModal.isOpen} onOpenChange={closeDeleteModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-red-400 dark:text-red-500">
              <Trash2 className="h-5 w-5" />
              Remover Instância
            </DialogTitle>
            <DialogDescription className="text-sidebar-foreground/70 dark:text-gray-400">
              Você está prestes a remover a instância{' '}
              <strong>{deleteModal.instance?.instanceName}</strong>. Esta ação
              não pode ser desfeita.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium text-sidebar-foreground dark:text-gray-400">
                Digite o nome da instância para confirmar:
              </label>
              <Input
                placeholder={deleteModal.instance?.instanceName || ''}
                value={deleteModal.confirmationText}
                onChange={(e) =>
                  setDeleteModal((prev) => ({
                    ...prev,
                    confirmationText: e.target.value,
                  }))
                }
                className="mt-2 bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50 dark:text-gray-400 dark:placeholder:text-gray-400"
              />
            </div>
          </div>

          <DialogFooter className="flex gap-2">
            <Button
              variant="outline"
              onClick={closeDeleteModal}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:bg-sidebar-accent"
            >
              Cancelar
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={
                !isDeleteConfirmationValid ||
                isDeleting === deleteModal.instance?.instanceName
              }
              className="bg-red-600 hover:bg-red-700 text-white dark:bg-red-600 dark:hover:bg-red-700"
            >
              {isDeleting === deleteModal.instance?.instanceName
                ? 'Removendo...'
                : 'Remover Instância'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
