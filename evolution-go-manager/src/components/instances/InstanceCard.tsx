/**
 * Instance Card Component
 * Displays an Evolution GO instance as a card with status and actions
 */

import { Button, Card, CardContent, Badge } from "@evoapi/design-system";
import {
  Settings,
  Trash2,
  Power,
  PowerOff,
  MessageSquare,
  FlaskConical,
  Network,
  RefreshCw,
} from "lucide-react";
import type { Instance } from "@/types/instance";
import TimelockTimer from "./TimelockTimer";

type InstanceCardProps = {
  instance: Instance;
  isDeleting?: string | null;
  isReconnecting?: string | null;
  onSettings: (instance: Instance) => void;
  onDelete: (instance: Instance) => void;
  onConnect: (instance: Instance) => void;
  onDisconnect: (instance: Instance) => void;
  onReconnect?: (instance: Instance) => void;
  onSendMessage?: (instance: Instance) => void;
  onTestMessage?: (instance: Instance) => void;
  onSetProxy?: (instance: Instance) => void;
};

const getStatusBadge = (status: string) => {
  const connected = status === "open";
  return (
    <Badge
      role="status"
      className={
        connected
          ? "bg-green-500/10 text-green-500 hover:bg-green-500/20"
          : "bg-red-500/10 text-red-500 hover:bg-red-500/20"
      }
    >
      {connected ? "Conectado" : "Desconectado"}
    </Badge>
  );
};

export default function InstanceCard({
  instance,
  isDeleting,
  isReconnecting,
  onSettings,
  onDelete,
  onConnect,
  onDisconnect,
  onReconnect,
  onSendMessage,
  onTestMessage,
  onSetProxy,
}: InstanceCardProps) {
  const isConnected = instance.status === "open";
  const isReconnectingThis = isReconnecting === instance.id;

  return (
    <Card className="group relative flex h-full flex-col bg-sidebar border-sidebar-border hover:bg-sidebar-accent/30 transition-all duration-300 hover:shadow-lg hover:shadow-black/10 overflow-hidden">
      <CardContent className="flex h-full flex-col p-0">
        {/* Header with icon, name and status */}
        <div className="flex items-center gap-3 p-4 border-b border-sidebar-border">
          <div className="flex-shrink-0">
            <div className="rounded-lg bg-sidebar-accent flex items-center justify-center w-14 h-14 overflow-hidden">
              {instance.profilePicUrl ? (
                <img
                  src={instance.profilePicUrl}
                  alt=""
                  aria-hidden="true"
                  loading="lazy"
                  className="w-14 h-14 object-cover rounded-lg"
                  onError={(e) => {
                    // Hide the broken image but keep the reserved slot, so the
                    // card height stays identical across the grid.
                    const target = e.target as HTMLImageElement;
                    target.style.visibility = "hidden";
                  }}
                />
              ) : (
                <span
                  aria-hidden="true"
                  className="text-lg font-semibold text-sidebar-foreground/70"
                >
                  {(instance.profileName || instance.instanceName || "?")
                    .charAt(0)
                    .toUpperCase()}
                </span>
              )}
            </div>
          </div>

          <div className="flex-1 min-w-0">
            <h3 className="font-semibold text-base truncate text-sidebar-foreground">
              {instance.profileName || instance.instanceName}
            </h3>
            <p className="text-xs text-sidebar-foreground/60 truncate">
              {instance.instanceName}
            </p>
          </div>

          <div className="flex-shrink-0">{getStatusBadge(instance.status)}</div>
        </div>

        {/* Details section */}
        <div className="flex-1 px-4 py-3 text-xs text-sidebar-foreground/70 space-y-1">
          <div className="flex items-center justify-between">
            <span>Status</span>
            <span className="font-mono">
              {isConnected ? "conectado" : "desconectado"}
            </span>
          </div>
          {instance.profileStatus && (
            <div className="flex items-center justify-between">
              <span>Recado</span>
              <span className="font-mono truncate ml-2 max-w-[150px]">
                {instance.profileStatus}
              </span>
            </div>
          )}
          {instance.owner && (
            <div className="flex items-center justify-between">
              <span>Proprietário</span>
              <span className="font-mono truncate ml-2 max-w-[150px]">
                {instance.owner}
              </span>
            </div>
          )}

          {instance.proxy?.host && (
            <div className="flex items-center justify-between">
              <span className="flex items-center gap-1">
                <Network className="h-3 w-3 text-blue-400" />
                Proxy
              </span>
              <span
                className="font-mono truncate ml-2 max-w-[150px] text-blue-400"
                title={`${instance.proxy.protocol || "http"}://${instance.proxy.host}:${instance.proxy.port}`}
              >
                {instance.proxy.host}:{instance.proxy.port}
              </span>
            </div>
          )}

          <TimelockTimer instanceId={instance.id} connected={isConnected} />
        </div>

        {/* Action buttons - hover effect */}
        <div className="flex border-t border-sidebar-border opacity-100 transition-opacity duration-200">
          {/* Connect/Disconnect Button */}
          {!isConnected && (
            <Button
              variant="ghost"
              className="flex-1 rounded-none h-12 text-green-500 hover:text-green-400 hover:bg-green-500/10"
              onClick={() => onConnect(instance)}
            >
              <Power className="h-4 w-4 mr-2" aria-hidden="true" />
              Conectar
            </Button>
          )}

          {isConnected && (
            <Button
              variant="ghost"
              className="flex-1 rounded-none h-12 text-yellow-500 hover:text-yellow-400 hover:bg-yellow-500/10"
              onClick={() => onDisconnect(instance)}
            >
              <PowerOff className="h-4 w-4 mr-2" aria-hidden="true" />
              Desconectar
            </Button>
          )}

          {isConnected && <div className="w-px bg-sidebar-border" />}

          {/* Reconnect Button - rebuilds the WhatsApp socket without a new QR */}
          {onReconnect && (
            <>
              <Button
                variant="ghost"
                className="rounded-none h-12 px-4 text-cyan-500 hover:text-cyan-400 hover:bg-cyan-500/10"
                disabled={isReconnectingThis}
                onClick={() => onReconnect(instance)}
                title="Reconectar instância"
              >
                <RefreshCw
                  className={`h-4 w-4 ${isReconnectingThis ? "animate-spin" : ""}`}
                />
              </Button>
              <div className="w-px bg-sidebar-border" />
            </>
          )}

          {/* Send Message Button - only show if connected */}
          {isConnected && onSendMessage && (
            <>
              <Button
                variant="ghost"
                className="rounded-none h-12 px-4 text-blue-500 hover:text-blue-400 hover:bg-blue-500/10"
                onClick={() => onSendMessage(instance)}
                title="Enviar mensagem de texto"
                aria-label={`Enviar mensagem de texto para ${instance.instanceName}`}
              >
                <MessageSquare className="h-4 w-4" aria-hidden="true" />
              </Button>
              <div className="w-px bg-sidebar-border" />
            </>
          )}

          {/* Test Interactive Messages Button - only show if connected */}
          {isConnected && onTestMessage && (
            <>
              <Button
                variant="ghost"
                className="rounded-none h-12 px-4 text-purple-500 hover:text-purple-400 hover:bg-purple-500/10"
                onClick={() => onTestMessage(instance)}
                title="Testar botoes, lista e carrossel"
                aria-label={`Testar mensagens interativas em ${instance.instanceName}`}
              >
                <FlaskConical className="h-4 w-4" aria-hidden="true" />
              </Button>
              <div className="w-px bg-sidebar-border" />
            </>
          )}

          {/* Proxy Button */}
          {onSetProxy && (
            <>
              <Button
                variant="ghost"
                className="rounded-none h-12 px-4 text-blue-400 hover:text-blue-300 hover:bg-blue-500/10"
                onClick={() => onSetProxy(instance)}
                title="Configurar proxy"
              >
                <Network className="h-4 w-4" />
              </Button>
              <div className="w-px bg-sidebar-border" />
            </>
          )}

          {/* Settings Button */}
          <Button
            variant="ghost"
            className="rounded-none h-12 px-4 text-gray-500 hover:text-gray-300 hover:bg-gray-500/10"
            onClick={() => onSettings(instance)}
            title="Configurações da instância"
            aria-label={`Configurações de ${instance.instanceName}`}
          >
            <Settings className="h-4 w-4" aria-hidden="true" />
          </Button>

          <div className="w-px bg-sidebar-border" />

          {/* Delete Button */}
          <Button
            variant="ghost"
            className="rounded-none h-12 px-4 text-red-500 hover:text-red-400 hover:bg-red-500/10"
            disabled={isDeleting === instance.instanceName}
            onClick={() => onDelete(instance)}
            title="Remover instância"
            aria-label={`Remover instância ${instance.instanceName}`}
          >
            <Trash2 className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
