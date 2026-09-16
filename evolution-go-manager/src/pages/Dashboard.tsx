/**
 * Dashboard
 *
 * Visão geral do servidor Evolution GO, construída exclusivamente a partir de
 * dados reais de `GET /instance/all` (via instancesStore). Não há números
 * simulados: cada métrica é derivada da lista de instâncias retornada pela API.
 */

import { useEffect, useMemo, useRef } from 'react';
import { Button, Card, CardContent, Skeleton } from '@evoapi/design-system';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Layers,
  Plug,
  PlugZap,
  RefreshCw,
  Smartphone,
  Webhook,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';

import useInstancesStore from '@/store/instancesStore';
import EmptyState from '@/components/base/EmptyState';
import type { Instance } from '@/types/instance';
import { cn } from '@/utils/cn';

/** Um valor de integração conta como ativo quando não é vazio/"false". */
const isEnabled = (value?: string | boolean) => {
  if (typeof value === 'boolean') return value;
  if (!value) return false;
  const normalized = value.trim().toLowerCase();
  return normalized !== '' && normalized !== 'false' && normalized !== '0';
};

const formatDate = (value?: string) => {
  if (!value) return '—';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '—';
  return parsed.toLocaleString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

interface StatCardProps {
  label: string;
  value: number;
  hint: string;
  icon: typeof Layers;
  tone?: 'default' | 'success' | 'danger';
}

function StatCard({ label, value, hint, icon: Icon, tone = 'default' }: StatCardProps) {
  const toneClasses = {
    default: 'bg-primary/10 text-primary',
    success: 'bg-green-500/10 text-green-500',
    danger: 'bg-red-500/10 text-red-500',
  }[tone];

  return (
    <Card className="bg-sidebar border-sidebar-border">
      <CardContent className="flex items-center gap-4 p-4 sm:p-5">
        <div className={cn('flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg', toneClasses)}>
          <Icon className="h-6 w-6" aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-sm font-medium text-sidebar-foreground/70 dark:text-gray-400">
            {label}
          </p>
          <p className="text-2xl font-bold tabular-nums text-sidebar-foreground dark:text-gray-200">
            {value}
          </p>
          <p className="truncate text-xs text-sidebar-foreground/60 dark:text-gray-400">
            {hint}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

export default function Dashboard() {
  const navigate = useNavigate();
  const { instances, isLoading, hasLoaded, isRefreshing, error, fetchInstances } =
    useInstancesStore();

  const initialFetchDone = useRef(false);

  useEffect(() => {
    if (!initialFetchDone.current) {
      fetchInstances();
      initialFetchDone.current = true;
    }

    // Refresh silencioso: mantém os cards montados enquanto os dados chegam.
    const interval = setInterval(() => {
      fetchInstances({ silent: true });
    }, 10000);

    return () => clearInterval(interval);
  }, [fetchInstances]);

  const stats = useMemo(() => {
    const total = instances.length;
    const connected = instances.filter((i: Instance) => i.connected).length;
    const disconnected = total - connected;

    const withWebhook = instances.filter((i: Instance) => isEnabled(i.webhook)).length;
    const withRabbit = instances.filter((i: Instance) => isEnabled(i.rabbitmqEnable)).length;
    const withWebsocket = instances.filter((i: Instance) => isEnabled(i.websocketEnable)).length;
    const withNats = instances.filter((i: Instance) => isEnabled(i.natsEnable)).length;
    const withAnyIntegration = instances.filter(
      (i: Instance) =>
        isEnabled(i.webhook) ||
        isEnabled(i.rabbitmqEnable) ||
        isEnabled(i.websocketEnable) ||
        isEnabled(i.natsEnable)
    ).length;

    const connectedPct = total > 0 ? Math.round((connected / total) * 100) : 0;

    const recent = [...instances]
      .sort((a, b) => {
        const aTime = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const bTime = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return bTime - aTime;
      })
      .slice(0, 5);

    const failing = instances.filter(
      (i: Instance) => !i.connected && Boolean(i.disconnectReason)
    );

    return {
      total,
      connected,
      disconnected,
      connectedPct,
      withWebhook,
      withRabbit,
      withWebsocket,
      withNats,
      withAnyIntegration,
      recent,
      failing,
    };
  }, [instances]);

  const integrations = [
    { label: 'Webhook', value: stats.withWebhook, icon: Webhook },
    { label: 'WebSocket', value: stats.withWebsocket, icon: Activity },
    { label: 'RabbitMQ', value: stats.withRabbit, icon: Plug },
    { label: 'NATS', value: stats.withNats, icon: PlugZap },
  ];

  const showSkeleton = isLoading && !hasLoaded;

  return (
    <div className="flex h-full flex-col p-4">
      {/* Header */}
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="flex-1">
          <h1 className="mb-2 text-2xl font-bold tracking-tight text-sidebar-foreground dark:text-gray-200">
            Dashboard
          </h1>
          <p className="text-sm text-sidebar-foreground/70 dark:text-gray-400">
            Visão geral das suas instâncias WhatsApp do Evolution GO
          </p>
        </div>
        <div className="flex-shrink-0">
          <Button
            variant="outline"
            onClick={() => fetchInstances()}
            disabled={isRefreshing}
            aria-label="Atualizar dados do dashboard"
            className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:bg-sidebar-accent"
          >
            <RefreshCw
              className={cn('mr-2 h-4 w-4', isRefreshing && 'animate-spin')}
              aria-hidden="true"
            />
            Atualizar
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-auto" aria-busy={showSkeleton}>
        {showSkeleton ? (
          <div role="status" aria-label="Carregando dashboard">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
              {Array.from({ length: 4 }).map((_, idx) => (
                <Skeleton key={idx} className="h-28 rounded-xl" />
              ))}
            </div>
            <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
              <Skeleton className="h-64 rounded-xl" />
              <Skeleton className="h-64 rounded-xl" />
            </div>
            <span className="sr-only">Carregando dados do dashboard…</span>
          </div>
        ) : error && instances.length === 0 ? (
          <div
            role="alert"
            className="flex h-full flex-col items-center justify-center px-4 py-12 text-center"
          >
            <div className="mb-4 rounded-full bg-red-500/10 p-6">
              <AlertTriangle className="h-12 w-12 text-red-500" aria-hidden="true" />
            </div>
            <h2 className="mb-2 text-lg font-semibold text-sidebar-foreground dark:text-gray-200">
              Não foi possível carregar o dashboard
            </h2>
            <p className="mb-6 max-w-md text-sm text-sidebar-foreground/60 dark:text-gray-400">
              {error}
            </p>
            <Button onClick={() => fetchInstances()}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              Tentar novamente
            </Button>
          </div>
        ) : stats.total === 0 ? (
          <EmptyState
            icon={Layers}
            title="Nenhuma instância ainda"
            description="Crie sua primeira instância para ver métricas de conexão e integrações aqui."
            action={{
              label: 'Ir para Instâncias',
              onClick: () => navigate('/manager/instances'),
            }}
            className="h-full"
          />
        ) : (
          <div className="space-y-6">
            {/* Stat cards */}
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <StatCard
                label="Instâncias"
                value={stats.total}
                hint="Total cadastrado"
                icon={Smartphone}
              />
              <StatCard
                label="Conectadas"
                value={stats.connected}
                hint={`${stats.connectedPct}% do total`}
                icon={CheckCircle2}
                tone="success"
              />
              <StatCard
                label="Desconectadas"
                value={stats.disconnected}
                hint={
                  stats.disconnected > 0
                    ? 'Precisam de reconexão'
                    : 'Tudo conectado'
                }
                icon={AlertTriangle}
                tone={stats.disconnected > 0 ? 'danger' : 'default'}
              />
              <StatCard
                label="Com integração"
                value={stats.withAnyIntegration}
                hint="Webhook, WS, RabbitMQ ou NATS"
                icon={Webhook}
              />
            </div>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              {/* Conectividade */}
              <Card className="bg-sidebar border-sidebar-border">
                <CardContent className="p-4 sm:p-5">
                  <h2 className="mb-1 text-base font-semibold text-sidebar-foreground dark:text-gray-200">
                    Conectividade
                  </h2>
                  <p className="mb-4 text-xs text-sidebar-foreground/60 dark:text-gray-400">
                    Proporção de instâncias online no momento
                  </p>

                  <div className="mb-2 flex items-baseline gap-2">
                    <span className="text-3xl font-bold tabular-nums text-sidebar-foreground dark:text-gray-200">
                      {stats.connectedPct}%
                    </span>
                    <span className="text-sm text-sidebar-foreground/60 dark:text-gray-400">
                      {stats.connected} de {stats.total} conectadas
                    </span>
                  </div>

                  <div
                    className="h-2.5 w-full overflow-hidden rounded-full bg-sidebar-accent"
                    role="progressbar"
                    aria-valuenow={stats.connectedPct}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-label="Percentual de instâncias conectadas"
                  >
                    <div
                      className="h-full rounded-full bg-green-500 transition-all duration-500"
                      style={{ width: `${stats.connectedPct}%` }}
                    />
                  </div>

                  <dl className="mt-5 grid grid-cols-2 gap-4">
                    <div className="rounded-lg bg-sidebar-accent/40 p-3">
                      <dt className="text-xs text-sidebar-foreground/60 dark:text-gray-400">
                        Online
                      </dt>
                      <dd className="text-xl font-semibold tabular-nums text-green-500">
                        {stats.connected}
                      </dd>
                    </div>
                    <div className="rounded-lg bg-sidebar-accent/40 p-3">
                      <dt className="text-xs text-sidebar-foreground/60 dark:text-gray-400">
                        Offline
                      </dt>
                      <dd className="text-xl font-semibold tabular-nums text-red-500">
                        {stats.disconnected}
                      </dd>
                    </div>
                  </dl>
                </CardContent>
              </Card>

              {/* Integrações */}
              <Card className="bg-sidebar border-sidebar-border">
                <CardContent className="p-4 sm:p-5">
                  <h2 className="mb-1 text-base font-semibold text-sidebar-foreground dark:text-gray-200">
                    Integrações
                  </h2>
                  <p className="mb-4 text-xs text-sidebar-foreground/60 dark:text-gray-400">
                    Quantas instâncias têm cada canal de eventos configurado
                  </p>

                  <ul className="space-y-3">
                    {integrations.map((integration) => {
                      const pct =
                        stats.total > 0
                          ? Math.round((integration.value / stats.total) * 100)
                          : 0;
                      return (
                        <li key={integration.label}>
                          <div className="mb-1.5 flex items-center justify-between gap-2 text-sm">
                            <span className="flex items-center gap-2 text-sidebar-foreground/80 dark:text-gray-300">
                              <integration.icon
                                className="h-4 w-4 flex-shrink-0 text-primary"
                                aria-hidden="true"
                              />
                              {integration.label}
                            </span>
                            <span className="tabular-nums text-sidebar-foreground/70 dark:text-gray-400">
                              {integration.value}
                              <span className="sr-only">
                                {' '}
                                de {stats.total} instâncias
                              </span>
                            </span>
                          </div>
                          <div
                            className="h-2 w-full overflow-hidden rounded-full bg-sidebar-accent"
                            role="progressbar"
                            aria-valuenow={pct}
                            aria-valuemin={0}
                            aria-valuemax={100}
                            aria-label={`${integration.label}: ${integration.value} de ${stats.total} instâncias`}
                          >
                            <div
                              className="h-full rounded-full bg-primary transition-all duration-500"
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                </CardContent>
              </Card>
            </div>

            {/* Instâncias recentes */}
            <Card className="bg-sidebar border-sidebar-border">
              <CardContent className="p-4 sm:p-5">
                <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <h2 className="text-base font-semibold text-sidebar-foreground dark:text-gray-200">
                      Instâncias recentes
                    </h2>
                    <p className="text-xs text-sidebar-foreground/60 dark:text-gray-400">
                      As últimas criadas neste servidor
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => navigate('/manager/instances')}
                    className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:bg-sidebar-accent"
                  >
                    Ver todas
                  </Button>
                </div>

                {/* Tabela em telas médias+, cartões empilhados no mobile */}
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[480px] border-collapse text-left text-sm">
                    <caption className="sr-only">
                      Instâncias criadas mais recentemente, com status e data de
                      criação
                    </caption>
                    <thead>
                      <tr className="border-b border-sidebar-border">
                        <th
                          scope="col"
                          className="pb-2 pr-4 font-medium text-sidebar-foreground/60 dark:text-gray-400"
                        >
                          Instância
                        </th>
                        <th
                          scope="col"
                          className="pb-2 pr-4 font-medium text-sidebar-foreground/60 dark:text-gray-400"
                        >
                          Status
                        </th>
                        <th
                          scope="col"
                          className="pb-2 font-medium text-sidebar-foreground/60 dark:text-gray-400"
                        >
                          Criada em
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {stats.recent.map((instance: Instance) => (
                        <tr
                          key={instance.id || instance.instanceName}
                          className="border-b border-sidebar-border/50 last:border-0"
                        >
                          <th
                            scope="row"
                            className="max-w-[220px] truncate py-3 pr-4 font-medium text-sidebar-foreground dark:text-gray-200"
                          >
                            {instance.instanceName}
                          </th>
                          <td className="py-3 pr-4">
                            <span
                              className={cn(
                                'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium',
                                instance.connected
                                  ? 'bg-green-500/10 text-green-500'
                                  : 'bg-red-500/10 text-red-500'
                              )}
                            >
                              <span
                                className={cn(
                                  'h-1.5 w-1.5 rounded-full',
                                  instance.connected ? 'bg-green-500' : 'bg-red-500'
                                )}
                                aria-hidden="true"
                              />
                              {instance.connected ? 'Conectada' : 'Desconectada'}
                            </span>
                          </td>
                          <td className="py-3 tabular-nums text-sidebar-foreground/70 dark:text-gray-400">
                            {formatDate(instance.createdAt)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>

            {/* Instâncias com falha reportada pela API */}
            {stats.failing.length > 0 && (
              <Card className="bg-sidebar border-sidebar-border">
                <CardContent className="p-4 sm:p-5">
                  <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-sidebar-foreground dark:text-gray-200">
                    <AlertTriangle
                      className="h-4 w-4 text-yellow-500"
                      aria-hidden="true"
                    />
                    Desconexões com motivo
                  </h2>
                  <p className="mb-4 text-xs text-sidebar-foreground/60 dark:text-gray-400">
                    Motivo informado pela API na última desconexão
                  </p>
                  <ul className="space-y-2">
                    {stats.failing.slice(0, 5).map((instance: Instance) => (
                      <li
                        key={instance.id || instance.instanceName}
                        className="flex flex-col gap-1 rounded-lg bg-sidebar-accent/40 p-3 sm:flex-row sm:items-center sm:justify-between"
                      >
                        <span className="truncate font-medium text-sidebar-foreground dark:text-gray-200">
                          {instance.instanceName}
                        </span>
                        <span className="truncate font-mono text-xs text-sidebar-foreground/70 dark:text-gray-400">
                          {instance.disconnectReason}
                        </span>
                      </li>
                    ))}
                  </ul>
                </CardContent>
              </Card>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
