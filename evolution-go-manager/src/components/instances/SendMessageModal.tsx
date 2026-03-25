import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { X } from 'lucide-react';
import { toast } from 'sonner';
import * as instancesApi from '@/services/api/instances';
import type { Instance } from '@/types/instance';

interface SendMessageModalProps {
  open: boolean;
  onClose: () => void;
  instance: Instance | null;
}

const sendMessageSchema = z.object({
  number: z.string().min(1, 'Número é obrigatório'),
  message: z.string().min(1, 'Mensagem é obrigatória'),
});

type SendMessageFormData = z.infer<typeof sendMessageSchema>;

function SendMessageModal({ open, onClose, instance }: SendMessageModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<SendMessageFormData>({
    resolver: zodResolver(sendMessageSchema),
  });

  const onSubmit = async (data: SendMessageFormData) => {
    if (!instance?.apikey) {
      toast.error('Token da instância não encontrado');
      return;
    }

    try {
      await instancesApi.sendMessage(instance.apikey, {
        number: data.number,
        text: data.message,
      });
      toast.success('Mensagem enviada com sucesso!');
      reset();
      onClose();
    } catch (error) {
      console.error('Send message error:', error);
      toast.error('Erro ao enviar mensagem');
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  if (!open || !instance) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg bg-card p-6 shadow-lg">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-foreground">
            Enviar Mensagem - {instance.instanceName}
          </h2>
          <button
            onClick={handleClose}
            className="rounded-md p-1 hover:bg-accent"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label
              htmlFor="number"
              className="mb-1 block text-sm font-medium text-foreground"
            >
              Número (com DDI)
            </label>
            <input
              id="number"
              type="text"
              placeholder="5511999999999"
              {...register('number')}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {errors.number && (
              <p className="mt-1 text-sm text-destructive">
                {errors.number.message}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="message"
              className="mb-1 block text-sm font-medium text-foreground"
            >
              Mensagem
            </label>
            <textarea
              id="message"
              rows={4}
              placeholder="Digite sua mensagem..."
              {...register('message')}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {errors.message && (
              <p className="mt-1 text-sm text-destructive">
                {errors.message.message}
              </p>
            )}
          </div>

          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleClose}
              disabled={isSubmitting}
              className="flex-1 rounded-md border border-input px-4 py-2 text-sm font-medium text-foreground hover:bg-accent disabled:opacity-50"
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {isSubmitting ? 'Enviando...' : 'Enviar'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default SendMessageModal;
