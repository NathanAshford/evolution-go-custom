/**
 * QRCode Modal Component
 * Displays QR code for WhatsApp connection
 */

import { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Button,
} from '@evoapi/design-system';
import { QrCode, RefreshCw, X, CheckCircle2, KeyRound, ExternalLink, Copy, Check } from 'lucide-react';
import { toast } from 'sonner';
import type { Instance } from '@/types/instance';
import { getQrCode } from '@/services/api/instances';

type PasskeyState = {
  stage: string;
  openUrl?: string;
  code?: string;
};

interface QRCodeModalProps {
  instance: Instance | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRefresh?: () => Promise<void>;
}

export default function QRCodeModal({
  instance,
  open,
  onOpenChange,
  onRefresh,
}: QRCodeModalProps) {
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [passkey, setPasskey] = useState<PasskeyState | null>(null);
  const [copied, setCopied] = useState(false);

  // Auto-refresh every 10 seconds to check connection status and update QR code
  useEffect(() => {
    if (!open || !instance || instance.connected) return;

    const interval = setInterval(() => {
      if (onRefresh) {
        console.log('Auto-refreshing QR Code and checking connection...');
        onRefresh().catch((err) => {
          console.error('Auto-refresh failed:', err);
        });
      }
    }, 10000); // 10 seconds (faster polling to detect connection)

    return () => clearInterval(interval);
  }, [open, instance, onRefresh]);

  // Poll the QR endpoint directly to detect a passkey ceremony (which replaces
  // the QR when the account requires a WebAuthn passkey to link).
  useEffect(() => {
    const token = instance?.apikey;
    if (!open || !instance || instance.connected || !token) {
      setPasskey(null);
      return;
    }
    let cancelled = false;
    const poll = () => {
      getQrCode(token)
        .then((res) => {
          if (cancelled) return;
          setPasskey(
            res.passkeyStage
              ? { stage: res.passkeyStage, openUrl: res.passkeyOpenUrl, code: res.passkeyCode }
              : null
          );
        })
        .catch(() => { /* keep last state */ });
    };
    poll();
    const interval = setInterval(poll, 4000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [open, instance]);

  const handleRefresh = async () => {
    if (!onRefresh) return;

    setIsRefreshing(true);
    try {
      await onRefresh();
      toast.success('QR Code atualizado!');
    } catch (error) {
      console.error('Erro ao atualizar QR Code:', error);
      toast.error('Erro ao atualizar QR Code');
    } finally {
      setIsRefreshing(false);
    }
  };

  if (!instance) return null;

  // WhatsApp shows the pairing code as two groups of four characters.
  const pairingCode = instance.qrcode?.pairingCode;
  const formattedPairingCode =
    pairingCode && pairingCode.length === 8
      ? `${pairingCode.slice(0, 4)}-${pairingCode.slice(4)}`
      : pairingCode;

  const handleCopyPairingCode = async () => {
    if (!pairingCode) return;
    try {
      await navigator.clipboard.writeText(pairingCode);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error('Não foi possível copiar o código');
    }
  };

  // If already connected, show success message
  if (instance.connected) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-green-500">
              <CheckCircle2 className="h-5 w-5" />
              Conectado com Sucesso!
            </DialogTitle>
            <DialogDescription className="text-sidebar-foreground/70">
              A instância {instance.instanceName} foi conectada ao WhatsApp.
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col items-center gap-4 py-6">
            <div className="rounded-full bg-green-500/10 p-4">
              <CheckCircle2 className="h-12 w-12 text-green-500" />
            </div>
            {instance.profileName && (
              <div className="text-center">
                <p className="text-sm text-sidebar-foreground/60">
                  Conectado como
                </p>
                <p className="text-lg font-semibold text-sidebar-foreground">
                  {instance.profileName}
                </p>
              </div>
            )}
          </div>

          <div className="flex justify-end">
            <Button
              onClick={() => onOpenChange(false)}
              className="w-full sm:w-auto"
            >
              Fechar
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  // Passkey ceremony in progress — this account requires a WebAuthn passkey to
  // link, so there is no QR to scan. Guide the operator to complete it in the
  // Evolution Passkey Helper browser extension on web.whatsapp.com.
  if (passkey) {
    const inConfirmation = passkey.stage === 'confirmation' && !!passkey.code;
    const done = passkey.stage === 'confirmed';
    const errored = passkey.stage === 'error';
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <KeyRound className="h-5 w-5 text-amber-500" />
              Verificação por Passkey
            </DialogTitle>
            <DialogDescription className="text-sidebar-foreground/70">
              Esta conta exige um <strong>passkey</strong> do WhatsApp para vincular a
              instância <strong>{instance.instanceName}</strong>. Abra o WhatsApp Web e
              conclua a verificação pela extensão <em>Evolution Passkey Helper</em>.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {passkey.openUrl && !done && !errored && (
              <Button
                onClick={() => window.open(passkey.openUrl!, '_blank', 'noopener')}
                className="w-full bg-amber-500 hover:bg-amber-400 text-black"
              >
                <ExternalLink className="mr-2 h-4 w-4" />
                Abrir WhatsApp Web
              </Button>
            )}

            {inConfirmation && (
              <div className="w-full rounded-lg bg-sidebar-accent p-3 text-center">
                <p className="text-xs text-sidebar-foreground/60">
                  Confirme que este código aparece no seu celular
                </p>
                <p className="mt-1 font-mono text-2xl font-semibold tracking-widest text-sidebar-foreground">
                  {passkey.code}
                </p>
              </div>
            )}

            <div className="rounded-lg bg-sidebar-accent p-4 text-sm text-sidebar-foreground/70">
              {errored ? (
                <p className="text-red-500">
                  Falha na cerimônia de passkey. Feche e tente conectar novamente.
                </p>
              ) : done ? (
                <p className="text-green-500">Concluindo o pareamento…</p>
              ) : (
                <ol className="space-y-1">
                  <li>1. Instale a extensão <em>Evolution Passkey Helper</em> no navegador</li>
                  <li>2. Clique em <strong>Abrir WhatsApp Web</strong></li>
                  <li>3. Autorize com seu passkey/biometria quando solicitado</li>
                  <li>4. Verifique o código e confirme na extensão</li>
                </ol>
              )}
            </div>

            <div className="flex justify-end">
              <Button
                variant="outline"
                onClick={() => onOpenChange(false)}
                className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
              >
                <X className="mr-2 h-4 w-4" />
                Fechar
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  // Show QR Code
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <QrCode className="h-5 w-5 text-primary" />
            Conectar WhatsApp
          </DialogTitle>
          <DialogDescription className="text-sidebar-foreground/70">
            Escaneie o QR Code abaixo com seu WhatsApp para conectar a instância{' '}
            <strong>{instance.instanceName}</strong>
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* QR Code Display */}
          <div className="flex flex-col items-center gap-4">
            {instance.qrcode?.base64 ? (
              <div className="rounded-lg border-2 border-sidebar-border bg-white p-4">
                <img
                  src={instance.qrcode.base64}
                  alt="QR Code"
                  className="h-64 w-64"
                />
              </div>
            ) : (
              <div className="flex h-64 w-64 items-center justify-center rounded-lg border-2 border-dashed border-sidebar-border bg-sidebar">
                <div className="text-center">
                  <QrCode className="mx-auto h-12 w-12 text-sidebar-foreground/40" />
                  <p className="mt-2 text-sm text-sidebar-foreground/60">
                    Aguardando QR Code...
                  </p>
                </div>
              </div>
            )}

            {/* Pairing Code (if available) */}
            {pairingCode && (
              <div className="w-full rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-center">
                <p className="text-xs text-sidebar-foreground/60">
                  Código de Pareamento
                </p>
                <div className="mt-1 flex items-center justify-center gap-2">
                  <p className="font-mono text-2xl font-semibold tracking-widest text-sidebar-foreground">
                    {formattedPairingCode}
                  </p>
                  <button
                    type="button"
                    onClick={handleCopyPairingCode}
                    aria-label="Copiar código de pareamento"
                    title="Copiar código"
                    className="text-sidebar-foreground/50 hover:text-sidebar-foreground transition-colors"
                  >
                    {copied ? (
                      <Check className="h-4 w-4 text-green-500" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </button>
                </div>
                <p className="mt-1 text-xs text-sidebar-foreground/60">
                  Digite no celular em Dispositivos conectados → Conectar com
                  número de telefone
                </p>
              </div>
            )}
          </div>

          {/* Instructions */}
          <div className="rounded-lg bg-sidebar-accent p-4">
            <p className="text-sm font-medium text-sidebar-foreground">
              Como conectar:
            </p>
            <ol className="mt-2 space-y-1 text-sm text-sidebar-foreground/70">
              <li>1. Abra o WhatsApp no seu celular</li>
              <li>2. Toque em Menu ou Configurações</li>
              <li>3. Toque em Dispositivos conectados</li>
              <li>4. Toque em Conectar um dispositivo</li>
              {pairingCode ? (
                <>
                  <li>5. Toque em Conectar com número de telefone</li>
                  <li>6. Digite o código de pareamento acima</li>
                </>
              ) : (
                <li>5. Aponte seu celular para esta tela para capturar o código</li>
              )}
            </ol>
          </div>

          {/* Actions */}
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={handleRefresh}
              disabled={isRefreshing}
              className="flex-1 bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
            >
              {isRefreshing ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Atualizando...
                </>
              ) : (
                <>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Atualizar QR Code
                </>
              )}
            </Button>
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
