/**
 * MCP page
 *
 * Shows the Model Context Protocol connection details so an operator can wire
 * Claude or ChatGPT to this server: the endpoint URL, the auth options, the
 * tools that are exposed, and a live connectivity check.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, CardContent, Badge } from '@evoapi/design-system';
import {
  Plug,
  Copy,
  Check,
  RefreshCw,
  Eye,
  EyeOff,
  CheckCircle2,
  XCircle,
  Wrench,
  KeyRound,
  Search,
} from 'lucide-react';
import { toast } from 'sonner';
import useAuthStore from '@/store/authStore';
import { fetchInstances } from '@/services/api/instances';
import type { Instance } from '@/types/instance';

type McpTool = { name: string; description: string };

type ProbeState = {
  status: 'idle' | 'checking' | 'ok' | 'error';
  message?: string;
  tools?: McpTool[];
  serverVersion?: string;
  protocolVersion?: string;
};

/** Small helper: a labelled value with a copy button. */
function CopyRow({
  label,
  value,
  hint,
  masked,
}: {
  label: string;
  value: string;
  hint?: string;
  masked?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(!masked);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error('Não foi possível copiar');
    }
  };

  const shown = revealed ? value : '•'.repeat(Math.min(value.length, 40));

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-sidebar-foreground/70">{label}</span>
        {hint && <span className="text-xs text-sidebar-foreground/40">{hint}</span>}
      </div>
      <div className="flex items-center gap-2">
        <code className="flex-1 min-w-0 overflow-x-auto whitespace-nowrap rounded-md border border-sidebar-border bg-sidebar px-3 py-2 font-mono text-xs text-sidebar-foreground">
          {shown || '—'}
        </code>
        {masked && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setRevealed((v) => !v)}
            aria-label={revealed ? 'Ocultar' : 'Exibir'}
            title={revealed ? 'Ocultar' : 'Exibir'}
            className="bg-sidebar border-sidebar-border"
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        )}
        <Button
          variant="outline"
          size="sm"
          onClick={copy}
          disabled={!value}
          aria-label="Copiar"
          title="Copiar"
          className="bg-sidebar border-sidebar-border"
        >
          {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}

function Section({
  title,
  icon: Icon,
  children,
  description,
}: {
  title: string;
  icon: React.ElementType;
  children: React.ReactNode;
  description?: string;
}) {
  return (
    <Card className="bg-sidebar border-sidebar-border">
      <CardContent className="p-5 space-y-4">
        <div>
          <h2 className="flex items-center gap-2 text-base font-semibold text-sidebar-foreground">
            <Icon className="h-4 w-4 text-primary" />
            {title}
          </h2>
          {description && (
            <p className="mt-1 text-xs text-sidebar-foreground/60">{description}</p>
          )}
        </div>
        {children}
      </CardContent>
    </Card>
  );
}

export default function Mcp() {
  const { apiUrl, apiKey } = useAuthStore();
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selectedId, setSelectedId] = useState<string>('');
  const [probe, setProbe] = useState<ProbeState>({ status: 'idle' });

  const baseUrl = useMemo(() => (apiUrl || '').replace(/\/$/, ''), [apiUrl]);
  const mcpUrl = `${baseUrl}/mcp`;

  useEffect(() => {
    fetchInstances()
      .then((list) => {
        setInstances(list);
        if (list.length > 0) setSelectedId((current) => current || list[0].id);
      })
      .catch(() => {
        // The page is still useful without the instance list — the admin key
        // form below works regardless.
        toast.error('Não foi possível carregar as instâncias');
      });
  }, []);

  const selected = instances.find((i) => i.id === selectedId);
  const instanceToken = selected?.apikey || '';

  // The token-in-path form is what most connectors accept, since many only take
  // a bare URL with no custom headers.
  const scopedUrl = instanceToken ? `${mcpUrl}/${instanceToken}` : '';

  const runProbe = useCallback(async () => {
    const key = instanceToken || apiKey;
    if (!key) {
      toast.error('Nenhuma chave disponível para testar');
      return;
    }

    setProbe({ status: 'checking' });
    try {
      const initRes = await fetch(mcpUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', apikey: key },
        body: JSON.stringify({
          jsonrpc: '2.0',
          id: 1,
          method: 'initialize',
          params: {
            protocolVersion: '2024-11-05',
            capabilities: {},
            clientInfo: { name: 'evolution-manager', version: '1.0' },
          },
        }),
      });
      const initJson = await initRes.json();
      if (initJson.error) throw new Error(initJson.error.message);

      const toolsRes = await fetch(mcpUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', apikey: key },
        body: JSON.stringify({ jsonrpc: '2.0', id: 2, method: 'tools/list' }),
      });
      const toolsJson = await toolsRes.json();
      if (toolsJson.error) throw new Error(toolsJson.error.message);

      setProbe({
        status: 'ok',
        tools: toolsJson.result?.tools ?? [],
        serverVersion: initJson.result?.serverInfo?.version,
        protocolVersion: initJson.result?.protocolVersion,
      });
      toast.success('Servidor MCP respondeu corretamente');
    } catch (error) {
      setProbe({
        status: 'error',
        message: error instanceof Error ? error.message : 'Falha ao contatar o servidor MCP',
      });
      toast.error('Falha ao contatar o servidor MCP');
    }
  }, [mcpUrl, instanceToken, apiKey]);

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold text-foreground">
            <Plug className="h-6 w-6 text-primary" />
            MCP
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Conecte o Claude ou o ChatGPT a este servidor para enviar mensagens e
            pesquisar conversas do WhatsApp.
          </p>
        </div>
        <Button onClick={runProbe} disabled={probe.status === 'checking'}>
          <RefreshCw
            className={`mr-2 h-4 w-4 ${probe.status === 'checking' ? 'animate-spin' : ''}`}
          />
          Testar conexão
        </Button>
      </div>

      {/* Connection status */}
      {probe.status !== 'idle' && (
        <Card
          className={`border ${
            probe.status === 'ok'
              ? 'border-green-500/30 bg-green-500/5'
              : probe.status === 'error'
                ? 'border-red-500/30 bg-red-500/5'
                : 'border-sidebar-border bg-sidebar'
          }`}
        >
          <CardContent className="flex items-center gap-3 p-4">
            {probe.status === 'ok' ? (
              <CheckCircle2 className="h-5 w-5 flex-shrink-0 text-green-500" />
            ) : probe.status === 'error' ? (
              <XCircle className="h-5 w-5 flex-shrink-0 text-red-500" />
            ) : (
              <RefreshCw className="h-5 w-5 flex-shrink-0 animate-spin text-muted-foreground" />
            )}
            <div className="text-sm">
              {probe.status === 'ok' && (
                <span className="text-sidebar-foreground">
                  Conectado — protocolo <strong>{probe.protocolVersion}</strong>, servidor{' '}
                  <strong>v{probe.serverVersion}</strong>, {probe.tools?.length ?? 0} ferramentas
                  disponíveis.
                </span>
              )}
              {probe.status === 'error' && (
                <span className="text-red-400">{probe.message}</span>
              )}
              {probe.status === 'checking' && (
                <span className="text-muted-foreground">Testando…</span>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Endpoint + credentials */}
        <Section
          title="Endpoint e credenciais"
          icon={KeyRound}
          description="JSON-RPC 2.0 sobre Streamable HTTP. Escolha uma instância para gerar a URL com escopo."
        >
          <div className="space-y-1">
            <label className="text-xs font-medium text-sidebar-foreground/70">
              Instância
            </label>
            <select
              value={selectedId}
              onChange={(e) => setSelectedId(e.target.value)}
              className="h-9 w-full rounded-md border border-sidebar-border bg-sidebar px-3 text-sm text-sidebar-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            >
              {instances.length === 0 && <option value="">Nenhuma instância</option>}
              {instances.map((instance) => (
                <option key={instance.id} value={instance.id}>
                  {instance.instanceName} {instance.connected ? '(conectada)' : '(desconectada)'}
                </option>
              ))}
            </select>
          </div>

          <CopyRow label="URL do servidor MCP" value={mcpUrl} />

          <CopyRow
            label="URL com token da instância"
            hint="recomendado — limita o assistente a esta instância"
            value={scopedUrl}
            masked
          />

          <CopyRow label="Token da instância" value={instanceToken} masked />

          <CopyRow
            label="Chave global (admin)"
            hint="acessa todas as instâncias"
            value={apiKey}
            masked
          />

          <div className="rounded-md border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-sidebar-foreground/80">
            A chave permite <strong>enviar mensagens em seu nome</strong>. Prefira o
            token da instância e trate-o como uma senha.
          </div>
        </Section>

        {/* Auth methods */}
        <Section
          title="Formas de autenticação"
          icon={Plug}
          description="Use a que o seu cliente suportar — todas equivalem."
        >
          <div className="space-y-2 text-xs">
            {[
              ['Header apikey', `apikey: ${instanceToken || '<TOKEN>'}`],
              ['Bearer token', `Authorization: Bearer ${instanceToken || '<TOKEN>'}`],
              ['Token na URL', `POST ${mcpUrl}/<TOKEN>`],
              ['Query string', `POST ${mcpUrl}?apikey=<TOKEN>`],
            ].map(([label, sample]) => (
              <div
                key={label}
                className="rounded-md border border-sidebar-border bg-sidebar p-2.5"
              >
                <div className="font-medium text-sidebar-foreground/70">{label}</div>
                <code className="mt-1 block overflow-x-auto whitespace-nowrap font-mono text-[11px] text-sidebar-foreground">
                  {sample}
                </code>
              </div>
            ))}
          </div>

          <div className="space-y-2 pt-2">
            <div className="text-xs font-medium text-sidebar-foreground/70">
              Escopo por chave
            </div>
            <ul className="space-y-1 text-xs text-sidebar-foreground/70">
              <li>
                <Badge className="mr-2 bg-green-500/10 text-green-500">Token da instância</Badge>
                o assistente só enxerga esta instância; o argumento{' '}
                <code className="font-mono">instance</code> é dispensável.
              </li>
              <li>
                <Badge className="mr-2 bg-amber-500/10 text-amber-500">Chave admin</Badge>
                acessa todas; o argumento <code className="font-mono">instance</code> passa a
                ser obrigatório.
              </li>
            </ul>
          </div>
        </Section>

        {/* How to connect */}
        <Section
          title="Como conectar"
          icon={Plug}
          description="Cole a URL com token no seu cliente."
        >
          <div className="space-y-3 text-xs">
            <div>
              <div className="font-medium text-sidebar-foreground">Claude (claude.ai / Desktop)</div>
              <p className="mt-1 text-sidebar-foreground/60">
                Configurações → Connectors → Add custom connector → cole a URL com token.
              </p>
            </div>
            <div>
              <div className="font-medium text-sidebar-foreground">Claude Code</div>
              <code className="mt-1 block overflow-x-auto whitespace-nowrap rounded-md border border-sidebar-border bg-sidebar p-2 font-mono text-[11px] text-sidebar-foreground">
                claude mcp add evolution-go --transport http {mcpUrl} --header "apikey:{' '}
                {instanceToken || '<TOKEN>'}"
              </code>
            </div>
            <div>
              <div className="font-medium text-sidebar-foreground">ChatGPT</div>
              <p className="mt-1 text-sidebar-foreground/60">
                Settings → Connectors → Create → MCP Server, com a mesma URL. As
                ferramentas <code className="font-mono">search</code> e{' '}
                <code className="font-mono">fetch</code> exigidas pelo deep research já
                estão implementadas.
              </p>
            </div>
          </div>
        </Section>

        {/* Tools */}
        <Section
          title="Ferramentas expostas"
          icon={Wrench}
          description={
            probe.tools
              ? `${probe.tools.length} ferramentas reportadas pelo servidor.`
              : 'Clique em "Testar conexão" para listar direto do servidor.'
          }
        >
          <div className="space-y-2">
            {(probe.tools && probe.tools.length > 0
              ? probe.tools
              : [
                  { name: 'list_instances', description: 'Lista instâncias e status de conexão' },
                  {
                    name: 'search_messages',
                    description:
                      'Busca por conteúdo, data, chat, remetente, direção ou tipo',
                  },
                  { name: 'get_chat_history', description: 'Lê mensagens recentes de uma conversa' },
                  { name: 'list_chats', description: 'Lista conversas mais ativas' },
                  { name: 'fetch_message', description: 'Busca uma mensagem pelo id' },
                  { name: 'search / fetch', description: 'Aliases do ChatGPT deep research' },
                  { name: 'send_text_message', description: 'Mensagem de texto' },
                  { name: 'send_media_message', description: 'Imagem, vídeo, áudio ou documento' },
                  { name: 'send_link_message', description: 'Link com preview' },
                  { name: 'send_location_message', description: 'Localização' },
                  { name: 'send_contact_message', description: 'Cartão de contato (vCard)' },
                  { name: 'send_sticker_message', description: 'Figurinha' },
                  { name: 'send_poll_message', description: 'Enquete' },
                  { name: 'send_buttons_message', description: 'Botões (reply/url/call/copy/pix)' },
                  { name: 'send_list_message', description: 'Menu de seleção' },
                  { name: 'send_carousel_message', description: 'Carrossel de cards' },
                  { name: 'send_status', description: 'Publica no status (stories)' },
                  { name: 'place_call / end_call', description: 'Liga e desliga (sem áudio)' },
                ]
            ).map((tool) => (
              <div
                key={tool.name}
                className="rounded-md border border-sidebar-border bg-sidebar p-2.5"
              >
                <code className="font-mono text-xs font-semibold text-primary">
                  {tool.name}
                </code>
                <p className="mt-0.5 text-xs text-sidebar-foreground/60">
                  {tool.description}
                </p>
              </div>
            ))}
          </div>
        </Section>

        {/* Search filters */}
        <Section
          title="Filtros de busca"
          icon={Search}
          description="Argumentos aceitos por search_messages."
        >
          <div className="grid gap-2 sm:grid-cols-2">
            {[
              ['query', 'Texto dentro da mensagem'],
              ['chat', 'Número ou JID da conversa'],
              ['sender', 'Quem enviou'],
              ['from_me', 'true = enviadas, false = recebidas'],
              ['is_group', 'true = grupos, false = diretas'],
              ['message_type', 'text, image, audio, document…'],
              ['start_date / end_date', 'YYYY-MM-DD ou RFC3339'],
              ['limit / offset', 'Paginação (máx. 500)'],
            ].map(([name, desc]) => (
              <div
                key={name}
                className="rounded-md border border-sidebar-border bg-sidebar p-2.5"
              >
                <code className="font-mono text-xs font-semibold text-sidebar-foreground">
                  {name}
                </code>
                <p className="mt-0.5 text-xs text-sidebar-foreground/60">{desc}</p>
              </div>
            ))}
          </div>

          <div className="rounded-md border border-sidebar-border bg-sidebar-accent p-3 text-xs text-sidebar-foreground/70">
            <strong>Atenção:</strong> a busca só enxerga mensagens trocadas depois que
            este recurso foi ativado — não há importação retroativa do histórico.
          </div>
        </Section>
      </div>
    </div>
  );
}
