/**
 * Create Instance Modal Component
 * Modal for creating a new WhatsApp instance
 */

import { useState } from 'react';
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
import { Plus, Loader2, ChevronDown, ChevronUp } from 'lucide-react';
import { toast } from 'sonner';
import * as instancesApi from '@/services/api/instances';
import useInstancesStore from '@/store/instancesStore';
import type { CreateInstancePayload } from '@/types/instance';
import { v4 as uuidv4 } from 'uuid';

interface CreateInstanceModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Validation schema
const createInstanceSchema = z.object({
  instanceName: z
    .string()
    .min(1, 'Nome da instância é obrigatório')
    .min(3, 'Nome deve ter pelo menos 3 caracteres')
    .max(50, 'Nome deve ter no máximo 50 caracteres')
    .regex(
      /^[a-zA-Z0-9-_]+$/,
      'Nome deve conter apenas letras, números, hífen e underscore'
    ),
  token: z.string().optional(),
  proxyHost: z.string().optional(),
  proxyPort: z.string().optional(),
  proxyUsername: z.string().optional(),
  proxyPassword: z.string().optional(),
});

type CreateInstanceFormData = z.infer<typeof createInstanceSchema>;

export default function CreateInstanceModal({
  open,
  onOpenChange,
}: CreateInstanceModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [showProxyConfig, setShowProxyConfig] = useState(false);
  const { addInstance, fetchInstances } = useInstancesStore();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CreateInstanceFormData>({
    resolver: zodResolver(createInstanceSchema),
    defaultValues: {
      instanceName: '',
      token: '',
      proxyHost: '',
      proxyPort: '',
      proxyUsername: '',
      proxyPassword: '',
    },
  });

  const onSubmit = async (data: CreateInstanceFormData) => {
    setIsLoading(true);

    try {
      const payload: CreateInstancePayload = {
        name: data.instanceName,
        token: data.token || uuidv4(), // Generate UUID if not provided
      };

      // Add proxy configuration if provided
      if (data.proxyHost && data.proxyPort) {
        payload.proxy = {
          host: data.proxyHost,
          port: data.proxyPort,
          username: data.proxyUsername,
          password: data.proxyPassword,
        };
      }

      const newInstance = await instancesApi.createInstance(payload);

      // Add to store
      addInstance(newInstance);

      toast.success('Instância criada com sucesso!', {
        description: `A instância "${data.instanceName}" foi criada.`,
      });

      // Refresh instances list
      await fetchInstances();

      // Close modal and reset form
      onOpenChange(false);
      reset();
    } catch (error) {
      console.error('Erro ao criar instância:', error);
      toast.error(
        error instanceof Error
          ? error.message
          : 'Erro ao criar instância. Tente novamente.'
      );
    } finally {
      setIsLoading(false);
    }
  };

  const handleClose = () => {
    if (!isLoading) {
      onOpenChange(false);
      reset();
      setShowProxyConfig(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Plus className="h-5 w-5 text-primary" />
            Nova Instância
          </DialogTitle>
          <DialogDescription className="text-sidebar-foreground/70">
            Crie uma nova instância WhatsApp para gerenciar suas conversas
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {/* Instance Name */}
          <div className="space-y-2">
            <Label htmlFor="instanceName">
              Nome da Instância <span className="text-red-500">*</span>
            </Label>
            <Input
              id="instanceName"
              type="text"
              placeholder="ex: minha-instancia"
              disabled={isLoading}
              {...register('instanceName')}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
            />
            {errors.instanceName && (
              <p className="text-destructive text-sm">
                {errors.instanceName.message}
              </p>
            )}
            <p className="text-xs text-sidebar-foreground/60">
              Use apenas letras, números, hífen (-) e underscore (_)
            </p>
          </div>

          {/* Token (Optional) */}
          <div className="space-y-2">
            <Label htmlFor="token">Token (Opcional)</Label>
            <Input
              id="token"
              type="text"
              placeholder="Token personalizado (UUID)"
              disabled={isLoading}
              {...register('token')}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
            />
            {errors.token && (
              <p className="text-destructive text-sm">
                {errors.token.message}
              </p>
            )}
            <p className="text-xs text-sidebar-foreground/60">
              Se não informado, será gerado um UUID automaticamente
            </p>
          </div>

          {/* Proxy Configuration (Collapsible) */}
          <div className="space-y-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setShowProxyConfig(!showProxyConfig)}
              disabled={isLoading}
              className="w-full justify-between bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
            >
              <span>Configuração de Proxy (Opcional)</span>
              {showProxyConfig ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </Button>

            {showProxyConfig && (
              <div className="space-y-4 pt-2 border-t border-sidebar-border">
                {/* Proxy Host */}
                <div className="space-y-2">
                  <Label htmlFor="proxyHost">Host do Proxy</Label>
                  <Input
                    id="proxyHost"
                    type="text"
                    placeholder="ex: proxy.example.com"
                    disabled={isLoading}
                    {...register('proxyHost')}
                    className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
                  />
                  {errors.proxyHost && (
                    <p className="text-destructive text-sm">
                      {errors.proxyHost.message}
                    </p>
                  )}
                </div>

                {/* Proxy Port */}
                <div className="space-y-2">
                  <Label htmlFor="proxyPort">Porta do Proxy</Label>
                  <Input
                    id="proxyPort"
                    type="text"
                    placeholder="ex: 8080"
                    disabled={isLoading}
                    {...register('proxyPort')}
                    className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
                  />
                  {errors.proxyPort && (
                    <p className="text-destructive text-sm">
                      {errors.proxyPort.message}
                    </p>
                  )}
                </div>

                {/* Proxy Username */}
                <div className="space-y-2">
                  <Label htmlFor="proxyUsername">Usuário (Opcional)</Label>
                  <Input
                    id="proxyUsername"
                    type="text"
                    placeholder="Usuário do proxy"
                    disabled={isLoading}
                    {...register('proxyUsername')}
                    className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
                  />
                  {errors.proxyUsername && (
                    <p className="text-destructive text-sm">
                      {errors.proxyUsername.message}
                    </p>
                  )}
                </div>

                {/* Proxy Password */}
                <div className="space-y-2">
                  <Label htmlFor="proxyPassword">Senha (Opcional)</Label>
                  <Input
                    id="proxyPassword"
                    type="password"
                    placeholder="Senha do proxy"
                    disabled={isLoading}
                    {...register('proxyPassword')}
                    className="bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50"
                  />
                  {errors.proxyPassword && (
                    <p className="text-destructive text-sm">
                      {errors.proxyPassword.message}
                    </p>
                  )}
                </div>
              </div>
            )}
          </div>

          <DialogFooter className="flex gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={handleClose}
              disabled={isLoading}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent"
            >
              Cancelar
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Criando...
                </>
              ) : (
                <>
                  <Plus className="mr-2 h-4 w-4" />
                  Criar Instância
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
