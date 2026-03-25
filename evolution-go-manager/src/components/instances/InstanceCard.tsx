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
} from "lucide-react";
import type { Instance } from "@/types/instance";

type InstanceCardProps = {
  instance: Instance;
  isDeleting?: string | null;
  onSettings: (instance: Instance) => void;
  onDelete: (instance: Instance) => void;
  onConnect: (instance: Instance) => void;
  onDisconnect: (instance: Instance) => void;
  onSendMessage?: (instance: Instance) => void;
};

const getStatusBadge = (status: string) => {
  if (status === "open") {
    return (
      <Badge className="bg-green-500/10 text-green-500 hover:bg-green-500/20">
        Conectado
      </Badge>
    );
  }

  return (
    <Badge className="bg-red-500/10 text-red-500 hover:bg-red-500/20">
      Desconectado
    </Badge>
  );
};

export default function InstanceCard({
  instance,
  isDeleting,
  onSettings,
  onDelete,
  onConnect,
  onDisconnect,
  onSendMessage,
}: InstanceCardProps) {
  const isConnected = instance.status === "open";

  return (
    <Card className="group relative bg-sidebar border-sidebar-border hover:bg-sidebar-accent/30 transition-all duration-300 hover:shadow-lg hover:shadow-black/10 overflow-hidden">
      <CardContent className="p-0">
        {/* Header with icon, name and status */}
        <div className="flex items-center gap-3 p-4 border-b border-sidebar-border">
          {instance.profilePicUrl && (
            <div className="flex-shrink-0 dark">
              <div className="rounded-lg bg-gray-900 flex items-center justify-center w-14 h-14 overflow-hidden">
                <img
                  src={instance.profilePicUrl}
                  alt={instance.profileName || instance.instanceName}
                  className="w-12 h-12 object-cover rounded-lg"
                  onError={(e) => {
                    const target = e.target as HTMLImageElement;
                    target.style.display = "none";
                  }}
                />
              </div>
            </div>
          )}

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
        <div className="px-4 py-3 text-xs text-sidebar-foreground/70 space-y-1">
          <div className="flex items-center justify-between">
            <span>Status</span>
            <span className="font-mono">{instance.status}</span>
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
        </div>

        {/* Action buttons - hover effect */}
        <div className="flex border-t border-sidebar-border opacity-0 group-hover:opacity-100 transition-opacity duration-200">
          {/* Connect/Disconnect Button */}
          {!isConnected && (
            <Button
              variant="ghost"
              className="flex-1 rounded-none h-12 text-green-500 hover:text-green-400 hover:bg-green-500/10"
              onClick={() => onConnect(instance)}
            >
              <Power className="h-4 w-4 mr-2" />
              Conectar
            </Button>
          )}

          {isConnected && (
            <Button
              variant="ghost"
              className="flex-1 rounded-none h-12 text-yellow-500 hover:text-yellow-400 hover:bg-yellow-500/10"
              onClick={() => onDisconnect(instance)}
            >
              <PowerOff className="h-4 w-4 mr-2" />
              Desconectar
            </Button>
          )}

          <div className="w-px bg-sidebar-border" />

          {/* Send Message Button - only show if connected */}
          {isConnected && onSendMessage && (
            <>
              <Button
                variant="ghost"
                className="rounded-none h-12 px-4 text-blue-500 hover:text-blue-400 hover:bg-blue-500/10"
                onClick={() => onSendMessage(instance)}
              >
                <MessageSquare className="h-4 w-4" />
              </Button>
              <div className="w-px bg-sidebar-border" />
            </>
          )}

          {/* Settings Button */}
          <Button
            variant="ghost"
            className="rounded-none h-12 px-4 text-gray-500 hover:text-gray-300 hover:bg-gray-500/10"
            onClick={() => onSettings(instance)}
          >
            <Settings className="h-4 w-4" />
          </Button>

          <div className="w-px bg-sidebar-border" />

          {/* Delete Button */}
          <Button
            variant="ghost"
            className="rounded-none h-12 px-4 text-red-500 hover:text-red-400 hover:bg-red-500/10"
            disabled={isDeleting === instance.instanceName}
            onClick={() => onDelete(instance)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
