import { useMemo, useState } from 'react';
import { X, FlaskConical, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import * as instancesApi from '@/services/api/instances';
import type { Instance } from '@/types/instance';

interface TestMessageModalProps {
  open: boolean;
  onClose: () => void;
  instance: Instance | null;
}

type TestScenarioId =
  | 'btn_reply_1'
  | 'btn_reply_3'
  | 'btn_copy'
  | 'btn_url'
  | 'btn_call'
  | 'btn_pix'
  | 'btn_cta_group'
  | 'list'
  | 'carousel_reply'
  | 'carousel_url'
  | 'carousel_call'
  | 'carousel_copy';

type TestScenario = {
  id: TestScenarioId;
  group: 'button' | 'list' | 'carousel';
  label: string;
  description: string;
  endpoint: 'button' | 'list' | 'carousel';
};

const SCENARIOS: TestScenario[] = [
  {
    id: 'btn_reply_1',
    group: 'button',
    endpoint: 'button',
    label: 'Reply (1 botao)',
    description: 'Um botao do tipo reply (quick_reply).',
  },
  {
    id: 'btn_reply_3',
    group: 'button',
    endpoint: 'button',
    label: 'Reply (3 botoes - limite)',
    description: 'Tres botoes reply. Limite maximo permitido pelo servidor.',
  },
  {
    id: 'btn_copy',
    group: 'button',
    endpoint: 'button',
    label: 'CTA Copy',
    description: 'Botao que copia um codigo para a area de transferencia.',
  },
  {
    id: 'btn_url',
    group: 'button',
    endpoint: 'button',
    label: 'CTA URL',
    description: 'Botao que abre um link no navegador.',
  },
  {
    id: 'btn_call',
    group: 'button',
    endpoint: 'button',
    label: 'CTA Call',
    description: 'Botao que inicia uma ligacao telefonica.',
  },
  {
    id: 'btn_pix',
    group: 'button',
    endpoint: 'button',
    label: 'PIX (sozinho)',
    description: 'Pagamento Pix. O servidor exige que seja o unico botao.',
  },
  {
    id: 'btn_cta_group',
    group: 'button',
    endpoint: 'button',
    label: 'CTAs agrupados (copy + url + call)',
    description:
      'Combinacao recomendada para aparecer no WhatsApp Web. NAO mistura com reply.',
  },
  {
    id: 'list',
    group: 'list',
    endpoint: 'list',
    label: 'Lista com secoes',
    description: 'Menu de selecao unica com duas secoes (Planos e Suporte).',
  },
  {
    id: 'carousel_reply',
    group: 'carousel',
    endpoint: 'carousel',
    label: 'Carrossel com botoes REPLY',
    description: '4 cards (Basico, Pro, Business, Enterprise) com botoes do tipo REPLY.',
  },
  {
    id: 'carousel_url',
    group: 'carousel',
    endpoint: 'carousel',
    label: 'Carrossel com botao URL',
    description:
      '4 cards (Site, Docs, GitHub, Comunidade). No carrossel o link vai no campo `id`.',
  },
  {
    id: 'carousel_call',
    group: 'carousel',
    endpoint: 'carousel',
    label: 'Carrossel com botao CALL',
    description:
      '3 cards (Atendimento, Suporte, Financeiro). No carrossel o telefone vai no campo `id`.',
  },
  {
    id: 'carousel_copy',
    group: 'carousel',
    endpoint: 'carousel',
    label: 'Carrossel com botao COPY',
    description: '4 cards com cupons distintos usando o campo `copyCode`.',
  },
];

const GROUP_LABELS: Record<TestScenario['group'], string> = {
  button: 'Botoes interativos (/send/button)',
  list: 'Lista (/send/list)',
  carousel: 'Carrossel (/send/carousel)',
};

function buildPayload(
  scenarioId: TestScenarioId,
  number: string,
): Record<string, unknown> {
  switch (scenarioId) {
    case 'btn_reply_1':
      return {
        number,
        title: 'Teste - Reply unico',
        description: 'Um botao do tipo reply.',
        footer: 'Evolution GO',
        buttons: [
          { type: 'reply', displayText: 'Confirmar', id: 'test_reply_1' },
        ],
      };
    case 'btn_reply_3':
      return {
        number,
        title: 'Teste - 3 Reply (limite)',
        description: 'Tres botoes reply.',
        footer: 'Evolution GO',
        buttons: [
          { type: 'reply', displayText: 'Opcao A', id: 'test_a' },
          { type: 'reply', displayText: 'Opcao B', id: 'test_b' },
          { type: 'reply', displayText: 'Opcao C', id: 'test_c' },
        ],
      };
    case 'btn_copy':
      return {
        number,
        title: 'Teste - CTA Copy',
        description: 'Botao COPY com codigo a ser copiado.',
        footer: 'Evolution GO',
        buttons: [
          {
            type: 'copy',
            displayText: 'Copiar cupom',
            copyCode: 'PROMO2026',
          },
        ],
      };
    case 'btn_url':
      return {
        number,
        title: 'Teste - CTA URL',
        description: 'Botao que abre um link.',
        footer: 'Evolution GO',
        buttons: [
          {
            type: 'url',
            displayText: 'Abrir site',
            url: 'https://evolutionapi.com',
          },
        ],
      };
    case 'btn_call':
      return {
        number,
        title: 'Teste - CTA Call',
        description: 'Botao que inicia uma ligacao.',
        footer: 'Evolution GO',
        buttons: [
          {
            type: 'call',
            displayText: 'Ligar agora',
            phoneNumber: '+' + number.replace(/\D/g, ''),
          },
        ],
      };
    case 'btn_pix':
      return {
        number,
        title: 'Teste - PIX',
        description: 'Botao Pix (envia sozinho).',
        footer: 'Evolution GO',
        buttons: [
          {
            type: 'pix',
            currency: 'BRL',
            name: 'Minha Loja',
            keyType: 'cpf',
            key: '12345678900',
          },
        ],
      };
    case 'btn_cta_group':
      return {
        number,
        title: 'Teste - CTAs agrupados',
        description: 'copy + url + call (funciona no WhatsApp Web).',
        footer: 'Evolution GO',
        buttons: [
          {
            type: 'copy',
            displayText: 'Copiar cupom',
            copyCode: 'CTA2026',
          },
          {
            type: 'url',
            displayText: 'Abrir site',
            url: 'https://evolutionapi.com',
          },
          {
            type: 'call',
            displayText: 'Ligar agora',
            phoneNumber: '+' + number.replace(/\D/g, ''),
          },
        ],
      };
    case 'list':
      return {
        number,
        title: 'Teste - Lista',
        description: 'Lista interativa com secoes e rows.',
        buttonText: 'Ver opcoes',
        footerText: 'Evolution GO',
        sections: [
          {
            title: 'Planos',
            rows: [
              {
                title: 'Plano Basico',
                description: 'R$ 29,90/mes',
                rowId: 'plan_basic',
              },
              {
                title: 'Plano Pro',
                description: 'R$ 59,90/mes',
                rowId: 'plan_pro',
              },
            ],
          },
          {
            title: 'Suporte',
            rows: [
              {
                title: 'Falar com atendente',
                description: 'Horario comercial',
                rowId: 'support_agent',
              },
              {
                title: 'Central de ajuda',
                description: 'Artigos e FAQ',
                rowId: 'support_kb',
              },
            ],
          },
        ],
      };
    case 'carousel_reply':
      return {
        number,
        body: 'Teste - Carrossel com botoes REPLY',
        footer: 'Evolution GO',
        cards: [
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/replyA/600/400',
            },
            body: { text: 'Card A - Plano Basico' },
            footer: 'R$ 29,90 / mes',
            buttons: [
              { type: 'REPLY', displayText: 'Assinar Basico', id: 'reply_basic' },
              { type: 'REPLY', displayText: 'Saber mais',     id: 'reply_basic_info' },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/replyB/600/400',
            },
            body: { text: 'Card B - Plano Pro' },
            footer: 'R$ 59,90 / mes',
            buttons: [
              { type: 'REPLY', displayText: 'Assinar Pro', id: 'reply_pro' },
              { type: 'REPLY', displayText: 'Saber mais',  id: 'reply_pro_info' },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/replyC/600/400',
            },
            body: { text: 'Card C - Plano Business' },
            footer: 'R$ 149,90 / mes',
            buttons: [
              { type: 'REPLY', displayText: 'Assinar Business', id: 'reply_business' },
              { type: 'REPLY', displayText: 'Saber mais',        id: 'reply_business_info' },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/replyD/600/400',
            },
            body: { text: 'Card D - Plano Enterprise' },
            footer: 'Sob consulta',
            buttons: [
              { type: 'REPLY', displayText: 'Falar com vendas', id: 'reply_enterprise' },
            ],
          },
        ],
      };
    case 'carousel_url':
      return {
        number,
        body: 'Teste - Carrossel com botao URL',
        footer: 'Evolution GO',
        cards: [
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/urlA/600/400',
            },
            body: { text: 'Card A - Site oficial' },
            footer: 'Abre o site principal',
            buttons: [
              {
                type: 'URL',
                displayText: 'Abrir site',
                id: 'https://evolutionapi.com',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/urlB/600/400',
            },
            body: { text: 'Card B - Documentacao' },
            footer: 'Abre os docs da API',
            buttons: [
              {
                type: 'URL',
                displayText: 'Ver documentacao',
                id: 'https://doc.evolutionapi.com',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/urlC/600/400',
            },
            body: { text: 'Card C - GitHub' },
            footer: 'Abre o repositorio',
            buttons: [
              {
                type: 'URL',
                displayText: 'Abrir GitHub',
                id: 'https://github.com/EvolutionAPI',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/urlD/600/400',
            },
            body: { text: 'Card D - Comunidade' },
            footer: 'Participe da comunidade',
            buttons: [
              {
                type: 'URL',
                displayText: 'Entrar na comunidade',
                id: 'https://evolutionapi.com/community',
              },
            ],
          },
        ],
      };
    case 'carousel_call':
      return {
        number,
        body: 'Teste - Carrossel com botao CALL',
        footer: 'Evolution GO',
        cards: [
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/callA/600/400',
            },
            body: { text: 'Card A - Atendimento geral' },
            footer: 'Horario comercial',
            buttons: [
              {
                type: 'CALL',
                displayText: 'Ligar - Atendimento',
                id: '+' + number.replace(/\D/g, ''),
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/callB/600/400',
            },
            body: { text: 'Card B - Suporte tecnico' },
            footer: '24x7',
            buttons: [
              {
                type: 'CALL',
                displayText: 'Ligar - Suporte',
                id: '+' + number.replace(/\D/g, ''),
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/callC/600/400',
            },
            body: { text: 'Card C - Financeiro' },
            footer: 'Seg a Sex, 9h-18h',
            buttons: [
              {
                type: 'CALL',
                displayText: 'Ligar - Financeiro',
                id: '+' + number.replace(/\D/g, ''),
              },
            ],
          },
        ],
      };
    case 'carousel_copy':
      return {
        number,
        body: 'Teste - Carrossel com botao COPY',
        footer: 'Evolution GO',
        cards: [
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/copyA/600/400',
            },
            body: { text: 'Card A - Cupom de primeira compra' },
            footer: '10% de desconto',
            buttons: [
              {
                type: 'COPY',
                displayText: 'Copiar cupom',
                copyCode: 'BEMVINDO10',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/copyB/600/400',
            },
            body: { text: 'Card B - Cupom Black Friday' },
            footer: '30% de desconto',
            buttons: [
              {
                type: 'COPY',
                displayText: 'Copiar cupom',
                copyCode: 'BLACK30',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/copyC/600/400',
            },
            body: { text: 'Card C - Cupom anual' },
            footer: '2 meses gratis',
            buttons: [
              {
                type: 'COPY',
                displayText: 'Copiar cupom',
                copyCode: 'ANUAL2MESES',
              },
            ],
          },
          {
            header: {
              imageUrl: 'https://picsum.photos/seed/copyD/600/400',
            },
            body: { text: 'Card D - Cupom VIP' },
            footer: 'Exclusivo para clientes',
            buttons: [
              {
                type: 'COPY',
                displayText: 'Copiar cupom VIP',
                copyCode: 'VIP2026',
              },
            ],
          },
        ],
      };
  }
}

function TestMessageModal({ open, onClose, instance }: TestMessageModalProps) {
  const [number, setNumber] = useState('');
  const [scenarioId, setScenarioId] =
    useState<TestScenarioId>('btn_reply_1');
  const [isSending, setIsSending] = useState(false);
  const [result, setResult] = useState<
    | { ok: true; messageId: string }
    | { ok: false; error: string }
    | null
  >(null);

  const scenario = useMemo(
    () => SCENARIOS.find((s) => s.id === scenarioId)!,
    [scenarioId],
  );

  const groupedScenarios = useMemo(() => {
    const groups: Record<TestScenario['group'], TestScenario[]> = {
      button: [],
      list: [],
      carousel: [],
    };
    for (const s of SCENARIOS) groups[s.group].push(s);
    return groups;
  }, []);

  const handleClose = () => {
    if (isSending) return;
    setResult(null);
    setNumber('');
    setScenarioId('btn_reply_1');
    onClose();
  };

  const handleSend = async () => {
    if (!instance?.apikey) {
      toast.error('Token da instancia nao encontrado');
      return;
    }

    const digits = number.replace(/\D/g, '');
    if (digits.length < 10) {
      toast.error('Numero invalido. Use o formato 55DDXXXXXXXXX.');
      return;
    }

    setIsSending(true);
    setResult(null);

    try {
      const payload = buildPayload(scenarioId, digits);
      let response;
      if (scenario.endpoint === 'button') {
        response = await instancesApi.sendButtonMessage(
          instance.apikey,
          payload,
        );
      } else if (scenario.endpoint === 'list') {
        response = await instancesApi.sendListMessage(
          instance.apikey,
          payload,
        );
      } else {
        response = await instancesApi.sendCarouselMessage(
          instance.apikey,
          payload,
        );
      }

      const messageId =
        (response.data as { Info?: { ID?: string } } | null)?.Info?.ID ||
        '(sem id)';
      setResult({ ok: true, messageId });
      toast.success('Teste enviado com sucesso!', {
        description: scenario.label,
      });
    } catch (err: unknown) {
      const axiosErr = err as {
        response?: { data?: { error?: string }; status?: number };
        message?: string;
      };
      const msg =
        axiosErr?.response?.data?.error ||
        axiosErr?.message ||
        'Erro desconhecido ao enviar.';
      setResult({ ok: false, error: msg });
      toast.error('Falha ao enviar teste', { description: msg });
    } finally {
      setIsSending(false);
    }
  };

  if (!open || !instance) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-lg bg-card p-6 shadow-lg">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-xl font-semibold text-foreground">
            <FlaskConical className="h-5 w-5 text-purple-500" />
            Testar mensagens - {instance.instanceName}
          </h2>
          <button
            onClick={handleClose}
            disabled={isSending}
            className="rounded-md p-1 hover:bg-accent disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label
              htmlFor="test-number"
              className="mb-1 block text-sm font-medium text-foreground"
            >
              Numero de destino (com DDI)
            </label>
            <input
              id="test-number"
              type="text"
              placeholder="5582988898565"
              disabled={isSending}
              value={number}
              onChange={(e) => setNumber(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <p className="mt-1 text-xs text-muted-foreground">
              Somente digitos. O DDI brasileiro e 55.
            </p>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-foreground">
              Modo de teste
            </label>
            <div className="space-y-3 rounded-md border border-input bg-background/50 p-3">
              {(Object.keys(groupedScenarios) as TestScenario['group'][]).map(
                (group) => (
                  <div key={group}>
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      {GROUP_LABELS[group]}
                    </p>
                    <div className="grid grid-cols-1 gap-1 sm:grid-cols-2">
                      {groupedScenarios[group].map((s) => (
                        <label
                          key={s.id}
                          className={`flex cursor-pointer items-start gap-2 rounded-md border p-2 text-sm transition-colors ${
                            scenarioId === s.id
                              ? 'border-primary bg-primary/10'
                              : 'border-transparent hover:bg-accent'
                          }`}
                        >
                          <input
                            type="radio"
                            name="scenario"
                            value={s.id}
                            checked={scenarioId === s.id}
                            disabled={isSending}
                            onChange={() => setScenarioId(s.id)}
                            className="mt-1"
                          />
                          <span className="flex-1">
                            <span className="block font-medium text-foreground">
                              {s.label}
                            </span>
                            <span className="block text-xs text-muted-foreground">
                              {s.description}
                            </span>
                          </span>
                        </label>
                      ))}
                    </div>
                  </div>
                ),
              )}
            </div>
          </div>

          {result && (
            <div
              className={`rounded-md border p-3 text-sm ${
                result.ok
                  ? 'border-green-500/40 bg-green-500/10 text-green-400'
                  : 'border-destructive/40 bg-destructive/10 text-destructive'
              }`}
            >
              {result.ok ? (
                <>
                  <p className="font-medium">Enviado com sucesso</p>
                  <p className="font-mono text-xs">
                    messageId: {result.messageId}
                  </p>
                </>
              ) : (
                <>
                  <p className="font-medium">Falha no envio</p>
                  <p className="text-xs">{result.error}</p>
                </>
              )}
            </div>
          )}

          <div className="flex gap-2 pt-2">
            <button
              type="button"
              onClick={handleClose}
              disabled={isSending}
              className="flex-1 rounded-md border border-input px-4 py-2 text-sm font-medium text-foreground hover:bg-accent disabled:opacity-50"
            >
              Fechar
            </button>
            <button
              type="button"
              onClick={handleSend}
              disabled={isSending || !number.trim()}
              className="flex-1 inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {isSending ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Enviando...
                </>
              ) : (
                <>
                  <FlaskConical className="h-4 w-4" />
                  Enviar teste
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default TestMessageModal;
