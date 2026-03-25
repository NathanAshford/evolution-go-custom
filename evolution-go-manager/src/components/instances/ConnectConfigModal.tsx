import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { X, ChevronDown, ChevronRight } from 'lucide-react';
import { useState } from 'react';
import type { Instance } from '@/types/instance';

interface ConnectConfigModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: (config: ConnectConfig) => void;
  instance: Instance | null;
}

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

const connectConfigSchema = z.object({
  webhookUrl: z.string().optional(),
  phone: z.string().optional(),
  rabbitmqEnable: z.string().optional(),
  websocketEnable: z.string().optional(),
  natsEnable: z.string().optional(),
  alwaysOnline: z.boolean().optional(),
  rejectCall: z.boolean().optional(),
  readMessages: z.boolean().optional(),
  ignoreGroups: z.boolean().optional(),
  ignoreStatus: z.boolean().optional(),
});

type ConnectConfigFormData = z.infer<typeof connectConfigSchema>;

const availableEvents = [
  'ALL',
  'MESSAGE',
  'READ_RECEIPT',
  'PRESENCE',
  'HISTORY_SYNC',
  'CHAT_PRESENCE',
  'CALL',
  'CONNECTION',
  'QRCODE',
  'LABEL',
  'CONTACT',
  'GROUP',
  'NEWSLETTER',
];

function ConnectConfigModal({ open, onClose, instance, onConfirm }: ConnectConfigModalProps) {
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showEvents, setShowEvents] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<ConnectConfigFormData>({
    resolver: zodResolver(connectConfigSchema),
  });

  const onSubmit = async (data: ConnectConfigFormData) => {
    const config: ConnectConfig = {
      webhookUrl: data.webhookUrl || '',
      subscribe: selectedEvents,
      phone: data.phone || '',
      rabbitmqEnable: data.rabbitmqEnable || '',
      websocketEnable: data.websocketEnable || '',
      natsEnable: data.natsEnable || '',
      alwaysOnline: data.alwaysOnline,
      rejectCall: data.rejectCall,
      readMessages: data.readMessages,
      ignoreGroups: data.ignoreGroups,
      ignoreStatus: data.ignoreStatus,
    };

    onConfirm(config);
    handleClose();
  };

  const handleClose = () => {
    reset();
    setSelectedEvents([]);
    setShowAdvanced(false);
    setShowEvents(false);
    onClose();
  };

  const toggleEvent = (event: string) => {
    setSelectedEvents(prev => {
      // Se clicar em ALL
      if (event === 'ALL') {
        // Se ALL já está selecionado, desmarcar tudo
        if (prev.includes('ALL')) {
          return [];
        }
        // Se não, selecionar apenas ALL
        return ['ALL'];
      }

      // Se clicar em outro evento e ALL está selecionado, remover ALL
      if (prev.includes('ALL')) {
        return [event];
      }

      // Toggle normal
      if (prev.includes(event)) {
        return prev.filter(e => e !== event);
      }
      return [...prev, event];
    });
  };

  const selectAllEvents = () => {
    setSelectedEvents(['ALL']);
  };

  const clearAllEvents = () => {
    setSelectedEvents([]);
  };

  if (!open || !instance) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-2xl rounded-lg bg-card p-6 shadow-lg max-h-[90vh] overflow-y-auto">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-foreground">
            Configurar Conexão - {instance.instanceName}
          </h2>
          <button
            onClick={handleClose}
            className="rounded-md p-1 hover:bg-accent"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {/* Webhook URL */}
          <div>
            <label
              htmlFor="webhookUrl"
              className="mb-1 block text-sm font-medium text-foreground"
            >
              Webhook URL (opcional)
            </label>
            <input
              id="webhookUrl"
              type="url"
              placeholder="https://seu-servidor.com/webhook"
              {...register('webhookUrl')}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {errors.webhookUrl && (
              <p className="mt-1 text-sm text-destructive">
                {errors.webhookUrl.message}
              </p>
            )}
          </div>

          {/* Events Selection */}
          <div>
            <button
              type="button"
              onClick={() => setShowEvents(!showEvents)}
              className="flex items-center gap-2 text-sm font-medium text-foreground hover:text-primary"
            >
              {showEvents ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              Eventos para Webhook ({selectedEvents.length} selecionados)
            </button>

            {showEvents && (
              <div className="mt-2 space-y-2 rounded-md border border-input p-3">
                <div className="flex gap-2 mb-2">
                  <button
                    type="button"
                    onClick={selectAllEvents}
                    className="text-xs px-2 py-1 rounded bg-primary text-primary-foreground hover:bg-primary/90"
                  >
                    Selecionar ALL
                  </button>
                  <button
                    type="button"
                    onClick={clearAllEvents}
                    className="text-xs px-2 py-1 rounded border border-input hover:bg-accent"
                  >
                    Limpar
                  </button>
                </div>

                {/* ALL Option - Highlighted */}
                <div className="mb-3 p-3 rounded-md bg-primary/10 border border-primary/20">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={selectedEvents.includes('ALL')}
                      onChange={() => toggleEvent('ALL')}
                      className="rounded border-input w-4 h-4"
                    />
                    <div className="flex-1">
                      <span className="text-sm font-semibold text-primary">ALL</span>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Seleciona todos os eventos (recomendado)
                      </p>
                    </div>
                  </label>
                </div>

                {/* Individual Events */}
                <div className="space-y-1">
                  <p className="text-xs font-medium text-muted-foreground mb-2">Eventos Individuais:</p>
                  <div className="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto">
                    {availableEvents.filter(e => e !== 'ALL').map((event) => (
                      <label
                        key={event}
                        className="flex items-center gap-2 text-sm cursor-pointer hover:bg-accent p-2 rounded"
                      >
                        <input
                          type="checkbox"
                          checked={selectedEvents.includes(event) || selectedEvents.includes('ALL')}
                          onChange={() => toggleEvent(event)}
                          disabled={selectedEvents.includes('ALL')}
                          className="rounded border-input"
                        />
                        <span className={selectedEvents.includes('ALL') ? 'text-muted-foreground' : 'text-foreground'}>
                          {event}
                        </span>
                      </label>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Pairing Code Phone */}
          <div>
            <label
              htmlFor="phone"
              className="mb-1 block text-sm font-medium text-foreground"
            >
              Telefone para Pairing Code (opcional)
            </label>
            <input
              id="phone"
              type="tel"
              placeholder="5511999999999"
              {...register('phone')}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <p className="mt-1 text-xs text-muted-foreground">
              Se fornecido, será gerado um código de pareamento (pairing code)
            </p>
          </div>

          {/* Advanced Settings */}
          <div>
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm font-medium text-foreground hover:text-primary"
            >
              {showAdvanced ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              Configurações Avançadas
            </button>

            {showAdvanced && (
              <div className="mt-2 space-y-4 rounded-md border border-input p-4">
                {/* Behavior Settings */}
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-foreground">Comportamento</h3>

                  <div className="flex items-center justify-between">
                    <div>
                      <label htmlFor="alwaysOnline" className="text-sm font-medium text-foreground cursor-pointer">
                        Always Online
                      </label>
                      <p className="text-xs text-muted-foreground">
                        Manter sempre online no WhatsApp
                      </p>
                    </div>
                    <input
                      id="alwaysOnline"
                      type="checkbox"
                      {...register('alwaysOnline')}
                      className="rounded border-input w-4 h-4"
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <label htmlFor="rejectCall" className="text-sm font-medium text-foreground cursor-pointer">
                        Reject Call
                      </label>
                      <p className="text-xs text-muted-foreground">
                        Rejeitar chamadas automaticamente
                      </p>
                    </div>
                    <input
                      id="rejectCall"
                      type="checkbox"
                      {...register('rejectCall')}
                      className="rounded border-input w-4 h-4"
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <label htmlFor="readMessages" className="text-sm font-medium text-foreground cursor-pointer">
                        Read Messages
                      </label>
                      <p className="text-xs text-muted-foreground">
                        Marcar mensagens como lidas
                      </p>
                    </div>
                    <input
                      id="readMessages"
                      type="checkbox"
                      {...register('readMessages')}
                      className="rounded border-input w-4 h-4"
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <label htmlFor="ignoreGroups" className="text-sm font-medium text-foreground cursor-pointer">
                        Ignore Groups
                      </label>
                      <p className="text-xs text-muted-foreground">
                        Ignorar mensagens de grupos
                      </p>
                    </div>
                    <input
                      id="ignoreGroups"
                      type="checkbox"
                      {...register('ignoreGroups')}
                      className="rounded border-input w-4 h-4"
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <label htmlFor="ignoreStatus" className="text-sm font-medium text-foreground cursor-pointer">
                        Ignore Status
                      </label>
                      <p className="text-xs text-muted-foreground">
                        Ignorar atualizações de status
                      </p>
                    </div>
                    <input
                      id="ignoreStatus"
                      type="checkbox"
                      {...register('ignoreStatus')}
                      className="rounded border-input w-4 h-4"
                    />
                  </div>
                </div>

                {/* Event Producers */}
                <div className="space-y-3 pt-3 border-t border-input">
                  <h3 className="text-sm font-semibold text-foreground">Produtores de Eventos</h3>

                  <div>
                    <label
                      htmlFor="rabbitmqEnable"
                      className="mb-1 block text-sm font-medium text-foreground"
                    >
                      RabbitMQ
                    </label>
                    <select
                      id="rabbitmqEnable"
                      {...register('rabbitmqEnable')}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="">Padrão</option>
                      <option value="enabled">Habilitado</option>
                      <option value="disabled">Desabilitado</option>
                    </select>
                  </div>

                  <div>
                    <label
                      htmlFor="websocketEnable"
                      className="mb-1 block text-sm font-medium text-foreground"
                    >
                      WebSocket
                    </label>
                    <select
                      id="websocketEnable"
                      {...register('websocketEnable')}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="">Padrão</option>
                      <option value="enabled">Habilitado</option>
                      <option value="disabled">Desabilitado</option>
                    </select>
                  </div>

                  <div>
                    <label
                      htmlFor="natsEnable"
                      className="mb-1 block text-sm font-medium text-foreground"
                    >
                      NATS
                    </label>
                    <select
                      id="natsEnable"
                      {...register('natsEnable')}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="">Padrão</option>
                      <option value="enabled">Habilitado</option>
                      <option value="disabled">Desabilitado</option>
                    </select>
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="flex gap-2 pt-4">
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-md border border-input px-4 py-2 text-sm font-medium text-foreground hover:bg-accent"
            >
              Cancelar
            </button>
            <button
              type="submit"
              className="flex-1 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            >
              Conectar
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default ConnectConfigModal;
