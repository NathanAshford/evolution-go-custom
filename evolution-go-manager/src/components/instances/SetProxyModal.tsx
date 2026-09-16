/**
 * SetProxyModal Component
 * Modal for setting, updating or removing proxy configuration on an existing instance
 */

import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Button,
  Input,
  Label,
} from '@evoapi/design-system';
import { Network, Loader2, Trash2, Eye, EyeOff, RefreshCw, PlugZap, CheckCircle2, XCircle, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import * as instancesApi from '@/services/api/instances';
import type { Instance } from '@/types/instance';
import type { ProxyTestResult } from '@/services/api/instances';

const proxySchema = z.object({
  host: z.string().min(1, 'Host é obrigatório'),
  port: z.string().min(1, 'Porta é obrigatória').regex(/^\d+$/, 'Porta deve ser numérica'),
  username: z.string().optional(),
  password: z.string().optional(),
  protocol: z.string().optional(),
});

type ProxyFormData = z.infer<typeof proxySchema>;

interface SetProxyModalProps {
  instance: Instance | null;
  open: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

const emptyProxy: ProxyFormData = {
  host: '',
  port: '',
  username: '',
  password: '',
  protocol: 'http',
};

export default function SetProxyModal({ instance, open, onClose, onSuccess }: SetProxyModalProps) {
  const [isSaving, setIsSaving] = useState(false);
  const [isRemoving, setIsRemoving] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [hasSavedProxy, setHasSavedProxy] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [testResult, setTestResult] = useState<ProxyTestResult | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    getValues,
    formState: { errors },
  } = useForm<ProxyFormData>({
    resolver: zodResolver(proxySchema),
    defaultValues: emptyProxy,
  });

  // Pre-fill with the proxy already saved for this instance so the operator can
  // see and edit it instead of retyping everything. The instance list already
  // carries the proxy, so it renders immediately; the authoritative values are
  // then re-fetched in case the list is stale.
  useEffect(() => {
    if (!open || !instance) return;

    setShowPassword(false);

    const fromList = instance.proxy;
    reset(fromList ? { ...emptyProxy, ...fromList } : emptyProxy);
    setHasSavedProxy(!!fromList?.host);

    let cancelled = false;
    setIsLoading(true);
    instancesApi
      .getInstanceProxy(instance.id)
      .then((saved) => {
        if (cancelled) return;
        reset(saved ? { ...emptyProxy, ...saved } : emptyProxy);
        setHasSavedProxy(!!saved?.host);
      })
      .catch((error) => {
        // Keep whatever came from the instance list — this is only a refresh.
        console.error('Erro ao carregar proxy salvo:', error);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, instance, reset]);

  const handleSave = async (data: ProxyFormData) => {
    if (!instance) return;
    setIsSaving(true);
    try {
      await instancesApi.setInstanceProxy(instance.id, {
        host: data.host,
        port: data.port,
        username: data.username || undefined,
        password: data.password || undefined,
        protocol: data.protocol || 'http',
      });
      toast.success(`Proxy configurado para ${instance.instanceName}. Reconectando...`);
      setHasSavedProxy(true);
      onSuccess?.();
      onClose();
    } catch (error) {
      console.error('Erro ao configurar proxy:', error);
      toast.error(error instanceof Error ? error.message : 'Erro ao configurar proxy');
    } finally {
      setIsSaving(false);
    }
  };

  const handleRemove = async () => {
    if (!instance) return;
    setIsRemoving(true);
    try {
      await instancesApi.removeInstanceProxy(instance.id);
      toast.success(`Proxy removido de ${instance.instanceName}. Reconectando sem proxy...`);
      reset(emptyProxy);
      setHasSavedProxy(false);
      onSuccess?.();
      onClose();
    } catch (error) {
      console.error('Erro ao remover proxy:', error);
      toast.error(error instanceof Error ? error.message : 'Erro ao remover proxy');
    } finally {
      setIsRemoving(false);
    }
  };

  // Rebuild the WhatsApp socket through the proxy already saved, without
  // touching the stored configuration.
  const handleReconnectProxy = async () => {
    if (!instance) return;
    setIsReconnecting(true);
    try {
      await instancesApi.reconnectInstanceProxy(instance.id);
      toast.success(`Reconectando ${instance.instanceName} pelo proxy...`);
      onSuccess?.();
    } catch (error) {
      console.error('Erro ao reconectar proxy:', error);
      toast.error(error instanceof Error ? error.message : 'Erro ao reconectar proxy');
    } finally {
      setIsReconnecting(false);
    }
  };

  // Probe the proxy currently typed into the form, falling back to the saved
  // one when the fields are empty. Nothing is saved and the live socket is not
  // touched, so this is safe to run on a connected instance.
  const handleTestProxy = async () => {
    if (!instance) return;
    setIsTesting(true);
    setTestResult(null);
    try {
      const values = getValues();
      const typed = values.host?.trim()
        ? {
            host: values.host.trim(),
            port: values.port.trim(),
            username: values.username?.trim() || '',
            password: values.password?.trim() || '',
            protocol: values.protocol || 'http',
          }
        : undefined;

      const result = await instancesApi.testInstanceProxy(instance.id, typed);
      setTestResult(result);

      if (result.ok) {
        toast.success(`Proxy funcionando — IP ${result.ip}`);
      } else {
        toast.error(result.error || 'Proxy não respondeu');
      }
    } catch (error) {
      console.error('Erro ao testar proxy:', error);
      const message = error instanceof Error ? error.message : 'Erro ao testar proxy';
      setTestResult({ ok: false, anonymous: false, whatsappReachable: false, error: message });
      toast.error(message);
    } finally {
      setIsTesting(false);
    }
  };

  const isBusy = isSaving || isRemoving || isReconnecting || isTesting;

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-sidebar-foreground">
            <Network className="h-5 w-5 text-blue-400" />
            Configurar Proxy
          </DialogTitle>
          <DialogDescription className="text-sidebar-foreground/70">
            Instância: <strong>{instance?.instanceName}</strong>
            <br />
            {isLoading
              ? 'Carregando proxy salvo...'
              : hasSavedProxy
                ? 'Proxy salvo carregado abaixo. Altere e salve, ou apenas reconecte.'
                : 'Nenhum proxy configurado. Preencha os campos para adicionar um.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(handleSave)} className="space-y-4">
          <div className="grid grid-cols-3 gap-3">
            <div className="col-span-2 space-y-1">
              <Label className="text-sidebar-foreground text-xs">Host *</Label>
              <Input
                placeholder="proxy.exemplo.com"
                {...register('host')}
                className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/40"
              />
              {errors.host && (
                <p className="text-xs text-red-400">{errors.host.message}</p>
              )}
            </div>
            <div className="space-y-1">
              <Label className="text-sidebar-foreground text-xs">Porta *</Label>
              <Input
                placeholder="8080"
                {...register('port')}
                className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/40"
              />
              {errors.port && (
                <p className="text-xs text-red-400">{errors.port.message}</p>
              )}
            </div>
          </div>

          <div className="space-y-1">
            <Label className="text-sidebar-foreground text-xs">Protocolo</Label>
            <select
              {...register('protocol')}
              className="w-full h-9 rounded-md border border-sidebar-border bg-sidebar px-3 text-sm text-sidebar-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
              <option value="socks4">SOCKS4</option>
            </select>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label className="text-sidebar-foreground text-xs">Usuário</Label>
              <Input
                placeholder="usuario"
                autoComplete="off"
                {...register('username')}
                className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/40"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-sidebar-foreground text-xs">Senha</Label>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  placeholder="••••••••"
                  autoComplete="new-password"
                  {...register('password')}
                  className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/40 pr-9"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? 'Ocultar senha' : 'Exibir senha'}
                  title={showPassword ? 'Ocultar senha' : 'Exibir senha'}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-sidebar-foreground/50 hover:text-sidebar-foreground transition-colors"
                >
                  {showPassword ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>
          </div>

          {testResult && (
            <div
              className={`rounded-md border p-3 text-sm ${
                testResult.ok
                  ? 'border-emerald-500/30 bg-emerald-500/10'
                  : 'border-red-500/30 bg-red-500/10'
              }`}
            >
              <div className="flex items-center gap-2 font-medium">
                {testResult.ok ? (
                  <>
                    <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                    <span className="text-emerald-300">Proxy funcionando</span>
                  </>
                ) : (
                  <>
                    <XCircle className="h-4 w-4 text-red-400" />
                    <span className="text-red-300">Proxy não respondeu</span>
                  </>
                )}
              </div>

              {testResult.ok ? (
                <div className="mt-2 space-y-1 text-sidebar-foreground/80">
                  <div className="flex justify-between gap-4">
                    <span>IP de saída</span>
                    <strong className="font-mono text-sidebar-foreground">{testResult.ip}</strong>
                  </div>
                  {testResult.latencyMs !== undefined && (
                    <div className="flex justify-between gap-4">
                      <span>Latência</span>
                      <strong className="text-sidebar-foreground">{testResult.latencyMs} ms</strong>
                    </div>
                  )}
                  {testResult.protocol && (
                    <div className="flex justify-between gap-4">
                      <span>Protocolo</span>
                      <strong className="text-sidebar-foreground">{testResult.protocol}</strong>
                    </div>
                  )}
                  <div className="flex justify-between gap-4">
                    <span>WhatsApp acessível</span>
                    <strong className={testResult.whatsappReachable ? 'text-emerald-300' : 'text-amber-300'}>
                      {testResult.whatsappReachable ? 'sim' : 'não'}
                    </strong>
                  </div>

                  {!testResult.anonymous && testResult.serverIp && (
                    <div className="mt-2 flex items-start gap-2 text-amber-300">
                      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                      <span>
                        O IP de saída é o mesmo do servidor ({testResult.serverIp}) — o tráfego
                        não está passando pelo proxy.
                      </span>
                    </div>
                  )}
                  {!testResult.whatsappReachable && (
                    <div className="mt-2 flex items-start gap-2 text-amber-300">
                      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                      <span>
                        O proxy responde, mas não alcançou o WhatsApp. A instância não vai
                        conectar por ele.
                      </span>
                    </div>
                  )}
                </div>
              ) : (
                <p className="mt-2 break-words text-red-200/90">{testResult.error}</p>
              )}
            </div>
          )}

          <DialogFooter className="flex flex-wrap gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={handleRemove}
              disabled={isBusy || !hasSavedProxy}
              className="text-red-400 border-red-400/30 hover:bg-red-500/10 hover:text-red-300"
            >
              {isRemoving ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Trash2 className="h-4 w-4 mr-2" />
              )}
              Remover Proxy
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={handleTestProxy}
              disabled={isBusy}
              title="Verifica se o proxy responde e mostra o IP de saída. Não salva nem reconecta."
              className="text-sky-400 border-sky-400/30 hover:bg-sky-500/10 hover:text-sky-300"
            >
              {isTesting ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <PlugZap className="h-4 w-4 mr-2" />
              )}
              Testar Proxy
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={handleReconnectProxy}
              disabled={isBusy || !hasSavedProxy}
              title={
                hasSavedProxy
                  ? 'Reconectar usando o proxy já salvo'
                  : 'Salve um proxy antes de reconectar'
              }
              className="text-amber-400 border-amber-400/30 hover:bg-amber-500/10 hover:text-amber-300"
            >
              {isReconnecting ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <RefreshCw className="h-4 w-4 mr-2" />
              )}
              Reconectar Proxy
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={isBusy}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
            >
              Cancelar
            </Button>

            <Button
              type="submit"
              disabled={isBusy || isLoading}
              className="bg-blue-600 hover:bg-blue-700 text-white"
            >
              {isSaving ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Network className="h-4 w-4 mr-2" />
              )}
              Salvar Proxy
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
