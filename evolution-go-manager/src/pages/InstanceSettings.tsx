import { useEffect, useState, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Save, Trash2, Power, Eye, EyeOff } from "lucide-react";
import { Button } from "@evoapi/design-system";
import { toast } from "sonner";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import * as instancesApi from "@/services/api/instances";
import type { Instance } from "@/types/instance";

const webhookSchema = z.object({
  webhookUrl: z.string().url("URL inválida").optional().or(z.literal("")),
  subscribe: z.array(z.string()).optional(),
  rabbitmqEnable: z.string().optional(),
  websocketEnable: z.string().optional(),
  natsEnable: z.string().optional(),
});

const advancedSchema = z.object({
  alwaysOnline: z.boolean().optional(),
  rejectCall: z.boolean().optional(),
  readMessages: z.boolean().optional(),
  ignoreGroups: z.boolean().optional(),
  ignoreStatus: z.boolean().optional(),
});

type WebhookFormData = z.infer<typeof webhookSchema>;
type AdvancedFormData = z.infer<typeof advancedSchema>;

const availableEvents = [
  "ALL",
  "MESSAGE",
  "READ_RECEIPT",
  "PRESENCE",
  "HISTORY_SYNC",
  "CHAT_PRESENCE",
  "CALL",
  "CONNECTION",
  "QRCODE",
  "LABEL",
  "CONTACT",
  "GROUP",
  "NEWSLETTER",
];

export default function InstanceSettings() {
  const { instanceId } = useParams<{ instanceId: string }>();
  const navigate = useNavigate();
  const [instance, setInstance] = useState<Instance | null>(null);
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [showToken, setShowToken] = useState(false);
  const isInitialized = useRef(false);
  const hasFetchedOnce = useRef(false);

  const {
    register: registerWebhook,
    handleSubmit: handleSubmitWebhook,
    formState: { errors: webhookErrors },
    reset: resetWebhook,
  } = useForm<WebhookFormData>({
    resolver: zodResolver(webhookSchema),
  });

  const {
    register: registerAdvanced,
    handleSubmit: handleSubmitAdvanced,
    reset: resetAdvanced,
  } = useForm<AdvancedFormData>({
    resolver: zodResolver(advancedSchema),
  });

  // Fetch instance data on mount (only once)
  useEffect(() => {
    const loadInstance = async () => {
      if (!instanceId || hasFetchedOnce.current) return;

      hasFetchedOnce.current = true;

      try {
        setIsLoading(true);
        const instanceData = await instancesApi.fetchInstance(instanceId);
        setInstance(instanceData);
      } catch (error) {
        console.error("Erro ao buscar instância:", error);
        toast.error("Erro ao carregar dados da instância");
      } finally {
        setIsLoading(false);
      }
    };

    loadInstance();
  }, [instanceId]);

  // Populate forms when instance data is loaded (only once)
  useEffect(() => {
    if (!instance || isInitialized.current) return;

    // Populate webhook form
    resetWebhook({
      webhookUrl: instance.webhook || "",
      rabbitmqEnable: instance.rabbitmqEnable || "",
      websocketEnable: instance.websocketEnable || "",
      natsEnable: instance.natsEnable || "",
    });

    // Populate advanced settings form
    resetAdvanced({
      alwaysOnline: instance.alwaysOnline || false,
      rejectCall: instance.rejectCall || false,
      readMessages: instance.readMessages || false,
      ignoreGroups: instance.ignoreGroups || false,
      ignoreStatus: instance.ignoreStatus || false,
    });

    // Parse events string to array (e.g., "MESSAGE,QRCODE,CONNECTION" -> ["MESSAGE", "QRCODE", "CONNECTION"])
    if (instance.events) {
      const eventsArray = instance.events
        .split(",")
        .map((e) => e.trim())
        .filter(Boolean);
      setSelectedEvents(eventsArray);
    } else {
      setSelectedEvents([]);
    }

    isInitialized.current = true;
  }, [instance, resetWebhook, resetAdvanced]);

  const toggleEvent = (event: string) => {
    setSelectedEvents((prev) => {
      if (event === "ALL") {
        if (prev.includes("ALL")) {
          return [];
        }
        return ["ALL"];
      }

      if (prev.includes("ALL")) {
        return [event];
      }

      if (prev.includes(event)) {
        return prev.filter((e) => e !== event);
      }
      return [...prev, event];
    });
  };

  const onSubmitWebhook = async (data: WebhookFormData) => {
    if (!instance?.apikey || !instanceId) {
      toast.error("Token da instância não encontrado");
      return;
    }

    try {
      setIsSaving(true);
      const config = {
        webhookUrl: data.webhookUrl || "",
        subscribe: selectedEvents,
        rabbitmqEnable: data.rabbitmqEnable || "",
        websocketEnable: data.websocketEnable || "",
        natsEnable: data.natsEnable || "",
      };

      await instancesApi.connectInstance(instance.apikey, config);
      toast.success("Configurações de webhook atualizadas!");

      // Refetch instance data
      const updatedInstance = await instancesApi.fetchInstance(instanceId);
      setInstance(updatedInstance);
    } catch (error) {
      console.error("Erro ao atualizar webhook:", error);
      toast.error(
        error instanceof Error ? error.message : "Erro ao atualizar webhook"
      );
    } finally {
      setIsSaving(false);
    }
  };

  const onSubmitAdvanced = async (data: AdvancedFormData) => {
    if (!instance?.apikey || !instance?.id || !instanceId) {
      toast.error("Token da instância não encontrado");
      return;
    }

    try {
      setIsSaving(true);
      await instancesApi.updateAdvancedSettings(
        instance.id,
        instance.apikey,
        data
      );
      toast.success("Configurações avançadas atualizadas!");

      // Refetch instance data
      const updatedInstance = await instancesApi.fetchInstance(instanceId);
      setInstance(updatedInstance);
    } catch (error) {
      console.error("Erro ao atualizar configurações:", error);
      toast.error(
        error instanceof Error
          ? error.message
          : "Erro ao atualizar configurações"
      );
    } finally {
      setIsSaving(false);
    }
  };

  const handleDisconnect = async () => {
    if (!instance?.apikey || !instanceId) {
      toast.error("Token da instância não encontrado");
      return;
    }

    try {
      toast.info(`Desconectando ${instance.instanceName}...`);
      await instancesApi.logoutInstance(instance.apikey);

      // Refetch instance data
      const updatedInstance = await instancesApi.fetchInstance(instanceId);
      setInstance(updatedInstance);

      toast.success(`${instance.instanceName} desconectada!`);
    } catch (error) {
      console.error("Erro ao desconectar instância:", error);
      toast.error(
        error instanceof Error ? error.message : "Erro ao desconectar instância"
      );
    }
  };

  const handleDelete = async () => {
    if (!instance?.id) {
      toast.error("ID da instância não encontrado");
      return;
    }

    const confirmed = window.confirm(
      `Tem certeza que deseja deletar a instância ${instance.instanceName}? Esta ação não pode ser desfeita.`
    );

    if (!confirmed) return;

    try {
      toast.info(`Deletando ${instance.instanceName}...`);
      await instancesApi.deleteInstance(instance.id);
      toast.success(`${instance.instanceName} deletada!`);
      navigate("/manager/instances");
    } catch (error) {
      console.error("Erro ao deletar instância:", error);
      toast.error(
        error instanceof Error ? error.message : "Erro ao deletar instância"
      );
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <p className="text-muted-foreground">Carregando...</p>
        </div>
      </div>
    );
  }

  if (!instance) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <h2 className="text-xl font-semibold text-foreground mb-2">
            Instância não encontrada
          </h2>
          <p className="text-muted-foreground mb-4">
            A instância "{instanceId}" não foi encontrada.
          </p>
          <Button onClick={() => navigate("/manager/instances")}>
            <ArrowLeft className="h-4 w-4 mr-2" />
            Voltar para Instâncias
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-sidebar-border bg-sidebar p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => navigate("/manager/instances")}
              className="text-sidebar-foreground hover:bg-sidebar-accent"
            >
              <ArrowLeft className="h-5 w-5" />
            </Button>
            <div>
              <h1 className="text-2xl font-bold text-foreground">
                Configurações
              </h1>
              <p className="text-sm text-muted-foreground">
                {instance.instanceName}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        <div className="max-w-4xl mx-auto space-y-6">
          {/* Instance Info Card */}
          <div className="rounded-lg border border-sidebar-border bg-card p-6">
            <h2 className="text-lg font-semibold text-foreground mb-4">
              Informações da Instância
            </h2>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium text-foreground">
                    Nome da Instância
                  </label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {instance.instanceName}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium text-foreground">
                    Token da Instância
                  </label>
                  <div className="mt-1 flex items-center gap-2">
                    <p className="text-sm text-muted-foreground font-mono">
                      {showToken ? (instance.apikey || '') : '•'.repeat((instance.apikey || '').length)}
                    </p>
                    <button
                      type="button"
                      onClick={() => setShowToken(!showToken)}
                      className="text-muted-foreground hover:text-foreground transition-colors"
                      title={showToken ? "Ocultar token" : "Mostrar token"}
                    >
                      {showToken ? <EyeOff size={18} /> : <Eye size={18} />}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="text-sm font-medium text-foreground">
                    Status
                  </label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {instance.status === "open" ? "Conectado" : "Desconectado"}
                  </p>
                </div>
                {instance.owner && (
                  <div>
                    <label className="text-sm font-medium text-foreground">
                      Número
                    </label>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {instance.owner}
                    </p>
                  </div>
                )}
                {instance.profileName && (
                  <div>
                    <label className="text-sm font-medium text-foreground">
                      Nome do Perfil
                    </label>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {instance.profileName}
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Webhook Settings Card */}
          <form onSubmit={handleSubmitWebhook(onSubmitWebhook)}>
            <div className="rounded-lg border border-sidebar-border bg-card p-6">
              <h2 className="text-lg font-semibold text-foreground mb-4">
                Configurações de Webhook
              </h2>
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor="webhookUrl"
                    className="block text-sm font-medium text-foreground mb-1"
                  >
                    URL do Webhook
                  </label>
                  <input
                    id="webhookUrl"
                    type="url"
                    placeholder="https://seu-servidor.com/webhook"
                    {...registerWebhook("webhookUrl")}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                  />
                  {webhookErrors.webhookUrl && (
                    <p className="mt-1 text-sm text-destructive">
                      {webhookErrors.webhookUrl.message}
                    </p>
                  )}
                  <p className="mt-1 text-xs text-muted-foreground">
                    URL que receberá os eventos do WhatsApp
                  </p>
                </div>

                {/* Events Selection */}
                <div>
                  <label className="block text-sm font-medium text-foreground mb-2">
                    Eventos para Webhook
                  </label>
                  <div className="space-y-2 rounded-md border border-input p-3 max-h-60 overflow-y-auto">
                    {/* ALL Option */}
                    <div className="p-2 rounded-md bg-primary/10 border border-primary/20">
                      <label className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={selectedEvents.includes("ALL")}
                          onChange={() => toggleEvent("ALL")}
                          className="rounded border-input w-4 h-4"
                        />
                        <span className="text-sm font-semibold text-primary">
                          ALL
                        </span>
                      </label>
                    </div>

                    {/* Individual Events */}
                    <div className="grid grid-cols-2 gap-2">
                      {availableEvents
                        .filter((e) => e !== "ALL")
                        .map((event) => (
                        <label
                          key={event}
                          className="flex items-center gap-2 text-sm cursor-pointer hover:bg-accent p-2 rounded"
                        >
                          <input
                            type="checkbox"
                              checked={
                                selectedEvents.includes(event) ||
                                selectedEvents.includes("ALL")
                              }
                            onChange={() => toggleEvent(event)}
                              disabled={selectedEvents.includes("ALL")}
                            className="rounded border-input"
                          />
                            <span
                              className={
                                selectedEvents.includes("ALL")
                                  ? "text-muted-foreground"
                                  : "text-foreground"
                              }
                            >
                            {event}
                          </span>
                        </label>
                      ))}
                    </div>
                  </div>
                </div>

                {/* Event Producers */}
                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <label
                      htmlFor="rabbitmqEnable"
                      className="block text-sm font-medium text-foreground mb-1"
                    >
                      RabbitMQ
                    </label>
                    <select
                      id="rabbitmqEnable"
                      {...registerWebhook("rabbitmqEnable")}
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
                      className="block text-sm font-medium text-foreground mb-1"
                    >
                      WebSocket
                    </label>
                    <select
                      id="websocketEnable"
                      {...registerWebhook("websocketEnable")}
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
                      className="block text-sm font-medium text-foreground mb-1"
                    >
                      NATS
                    </label>
                    <select
                      id="natsEnable"
                      {...registerWebhook("natsEnable")}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                      <option value="">Padrão</option>
                      <option value="enabled">Habilitado</option>
                      <option value="disabled">Desabilitado</option>
                    </select>
                  </div>
                </div>

                <div className="flex justify-end">
                  <Button type="submit" disabled={isSaving} className="gap-2">
                    <Save className="h-4 w-4" />
                    {isSaving ? "Salvando..." : "Salvar Webhook"}
                  </Button>
                </div>
              </div>
            </div>
          </form>

          {/* Advanced Settings Card */}
          <form onSubmit={handleSubmitAdvanced(onSubmitAdvanced)}>
            <div className="rounded-lg border border-sidebar-border bg-card p-6">
              <h2 className="text-lg font-semibold text-foreground mb-4">
                Configurações Avançadas
              </h2>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <label
                      htmlFor="alwaysOnline"
                      className="text-sm font-medium text-foreground cursor-pointer"
                    >
                      Always Online
                    </label>
                    <p className="text-xs text-muted-foreground">
                      Manter sempre online no WhatsApp
                    </p>
                  </div>
                  <input
                    id="alwaysOnline"
                    type="checkbox"
                    {...registerAdvanced("alwaysOnline")}
                    className="rounded border-input w-4 h-4"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label
                      htmlFor="rejectCall"
                      className="text-sm font-medium text-foreground cursor-pointer"
                    >
                      Reject Call
                    </label>
                    <p className="text-xs text-muted-foreground">
                      Rejeitar chamadas automaticamente
                    </p>
                  </div>
                  <input
                    id="rejectCall"
                    type="checkbox"
                    {...registerAdvanced("rejectCall")}
                    className="rounded border-input w-4 h-4"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label
                      htmlFor="readMessages"
                      className="text-sm font-medium text-foreground cursor-pointer"
                    >
                      Read Messages
                    </label>
                    <p className="text-xs text-muted-foreground">
                      Marcar mensagens como lidas
                    </p>
                  </div>
                  <input
                    id="readMessages"
                    type="checkbox"
                    {...registerAdvanced("readMessages")}
                    className="rounded border-input w-4 h-4"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label
                      htmlFor="ignoreGroups"
                      className="text-sm font-medium text-foreground cursor-pointer"
                    >
                      Ignore Groups
                    </label>
                    <p className="text-xs text-muted-foreground">
                      Ignorar mensagens de grupos
                    </p>
                  </div>
                  <input
                    id="ignoreGroups"
                    type="checkbox"
                    {...registerAdvanced("ignoreGroups")}
                    className="rounded border-input w-4 h-4"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label
                      htmlFor="ignoreStatus"
                      className="text-sm font-medium text-foreground cursor-pointer"
                    >
                      Ignore Status
                    </label>
                    <p className="text-xs text-muted-foreground">
                      Ignorar atualizações de status
                    </p>
                  </div>
                  <input
                    id="ignoreStatus"
                    type="checkbox"
                    {...registerAdvanced("ignoreStatus")}
                    className="rounded border-input w-4 h-4"
                  />
                </div>

                <div className="flex justify-end">
                  <Button type="submit" disabled={isSaving} className="gap-2">
                    <Save className="h-4 w-4" />
                    {isSaving ? "Salvando..." : "Salvar Avançadas"}
                  </Button>
                </div>
              </div>
            </div>
          </form>

          {/* Danger Zone Card */}
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6">
            <h2 className="text-lg font-semibold text-destructive mb-4">
              Zona de Perigo
            </h2>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-medium text-foreground">
                    Desconectar Instância
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    Desconecta a instância do WhatsApp
                  </p>
                </div>
                <Button
                  variant="destructive"
                  onClick={handleDisconnect}
                  className="gap-2"
                >
                  <Power className="h-4 w-4" />
                  Desconectar
                </Button>
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-medium text-foreground">
                    Deletar Instância
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    Remove permanentemente esta instância
                  </p>
                </div>
                <Button
                  variant="destructive"
                  onClick={handleDelete}
                  className="gap-2"
                >
                  <Trash2 className="h-4 w-4" />
                  Deletar
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
