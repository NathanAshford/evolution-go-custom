import { useCallback, useEffect, useMemo, useState } from 'react';
import axios, { AxiosHeaders } from 'axios';
import { toast } from 'sonner';
import {
  Play,
  Loader2,
  ChevronRight,
  Search as SearchIcon,
  RefreshCcw,
  Code2,
  FileJson,
  Clock,
  KeyRound,
} from 'lucide-react';
import useAuth from '@/hooks/useAuth';
import useInstancesStore from '@/store/instancesStore';

type OpenApiParameter = {
  name: string;
  in: 'path' | 'query' | 'header' | 'body';
  required?: boolean;
  type?: string;
  description?: string;
  schema?: { $ref?: string; type?: string };
};

type OpenApiOperation = {
  tags?: string[];
  summary?: string;
  description?: string;
  parameters?: OpenApiParameter[];
  responses?: Record<string, unknown>;
};

type OpenApiSpec = {
  paths: Record<string, Record<string, OpenApiOperation>>;
  definitions?: Record<
    string,
    {
      type?: string;
      properties?: Record<
        string,
        {
          type?: string;
          description?: string;
          example?: unknown;
          enum?: unknown[];
          items?: { $ref?: string; type?: string };
          $ref?: string;
        }
      >;
      required?: string[];
    }
  >;
};

type EndpointEntry = {
  path: string;
  method: string;
  operation: OpenApiOperation;
  tag: string;
  key: string;
};

const METHOD_COLOR: Record<string, string> = {
  get: 'bg-emerald-500/15 text-emerald-500 border-emerald-500/30',
  post: 'bg-blue-500/15 text-blue-500 border-blue-500/30',
  put: 'bg-amber-500/15 text-amber-500 border-amber-500/30',
  patch: 'bg-cyan-500/15 text-cyan-500 border-cyan-500/30',
  delete: 'bg-red-500/15 text-red-500 border-red-500/30',
};

/**
 * Resolve a $ref path like "#/definitions/foo.Bar" against the spec.
 */
function resolveRef(spec: OpenApiSpec | null, ref?: string) {
  if (!spec || !ref || !ref.startsWith('#/definitions/')) return null;
  const name = ref.replace('#/definitions/', '');
  return spec.definitions?.[name] || null;
}

/**
 * Build a JSON skeleton from a definition with example values from the schema.
 * Handles nested $refs and arrays. Cycles/depth guarded.
 */
function buildExampleFromDefinition(
  spec: OpenApiSpec | null,
  ref?: string,
  visited = new Set<string>(),
  depth = 0,
): unknown {
  if (!ref || depth > 5) return null;
  if (visited.has(ref)) return null;
  visited.add(ref);

  const def = resolveRef(spec, ref);
  if (!def || !def.properties) return null;

  const out: Record<string, unknown> = {};
  for (const [propName, propSchema] of Object.entries(def.properties)) {
    if (propSchema.example !== undefined) {
      out[propName] = propSchema.example;
      continue;
    }
    if (propSchema.$ref) {
      out[propName] = buildExampleFromDefinition(
        spec,
        propSchema.$ref,
        new Set(visited),
        depth + 1,
      );
      continue;
    }
    if (propSchema.type === 'array' && propSchema.items) {
      if (propSchema.items.$ref) {
        const item = buildExampleFromDefinition(
          spec,
          propSchema.items.$ref,
          new Set(visited),
          depth + 1,
        );
        out[propName] = item !== null ? [item] : [];
      } else {
        out[propName] = [];
      }
      continue;
    }
    if (propSchema.enum && propSchema.enum.length > 0) {
      out[propName] = propSchema.enum[0];
      continue;
    }
    switch (propSchema.type) {
      case 'string':
        out[propName] = '';
        break;
      case 'integer':
      case 'number':
        out[propName] = 0;
        break;
      case 'boolean':
        out[propName] = false;
        break;
      default:
        out[propName] = null;
    }
  }
  return out;
}

/**
 * Render any JSON-ish value into a pretty string.
 */
function pretty(v: unknown): string {
  try {
    return typeof v === 'string' ? v : JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

type ExecResult = {
  status: number;
  statusText: string;
  durationMs: number;
  data: unknown;
  headers: Record<string, string>;
  finalUrl: string;
  request: {
    method: string;
    url: string;
    headers: Record<string, string>;
    body?: unknown;
  };
};

function ApiTester() {
  const { apiUrl, apiKey: globalApiKey } = useAuth();
  const { instances, fetchInstances } = useInstancesStore();

  const [spec, setSpec] = useState<OpenApiSpec | null>(null);
  const [loadingSpec, setLoadingSpec] = useState(true);
  const [specError, setSpecError] = useState<string | null>(null);

  const [query, setQuery] = useState('');
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  const [pathParams, setPathParams] = useState<Record<string, string>>({});
  const [queryParams, setQueryParams] = useState<Record<string, string>>({});
  const [bodyText, setBodyText] = useState('');
  const [customHeaders, setCustomHeaders] = useState('');

  type ApikeyMode = 'global' | 'instance' | 'custom';
  const [apikeyMode, setApikeyMode] = useState<ApikeyMode>('global');
  const [selectedInstanceId, setSelectedInstanceId] = useState('');
  const [customApikey, setCustomApikey] = useState('');

  const [executing, setExecuting] = useState(false);
  const [result, setResult] = useState<ExecResult | null>(null);
  const [resultError, setResultError] = useState<string | null>(null);

  // Fetch swagger spec
  const loadSpec = useCallback(async () => {
    setLoadingSpec(true);
    setSpecError(null);
    try {
      const target = apiUrl
        ? `${apiUrl.replace(/\/$/, '')}/swagger/doc.json`
        : '/swagger/doc.json';
      const res = await axios.get<OpenApiSpec>(target, {
        headers: { 'Cache-Control': 'no-cache' },
      });
      setSpec(res.data);
    } catch (err) {
      console.error('Failed to load swagger spec:', err);
      setSpecError(
        'Nao foi possivel carregar /swagger/doc.json. Verifique se o servidor esta online.',
      );
    } finally {
      setLoadingSpec(false);
    }
  }, [apiUrl]);

  useEffect(() => {
    loadSpec();
  }, [loadSpec]);

  useEffect(() => {
    fetchInstances();
  }, [fetchInstances]);

  // Build flat endpoint list
  const endpoints: EndpointEntry[] = useMemo(() => {
    if (!spec) return [];
    const out: EndpointEntry[] = [];
    for (const [path, byMethod] of Object.entries(spec.paths)) {
      for (const [method, op] of Object.entries(byMethod)) {
        if (
          !['get', 'post', 'put', 'patch', 'delete', 'options'].includes(
            method,
          )
        ) {
          continue;
        }
        out.push({
          path,
          method: method.toUpperCase(),
          operation: op,
          tag: op.tags?.[0] || 'Outros',
          key: `${method.toUpperCase()} ${path}`,
        });
      }
    }
    return out.sort((a, b) => a.tag.localeCompare(b.tag) || a.path.localeCompare(b.path));
  }, [spec]);

  const filteredEndpoints = useMemo(() => {
    if (!query.trim()) return endpoints;
    const q = query.toLowerCase();
    return endpoints.filter(
      (e) =>
        e.path.toLowerCase().includes(q) ||
        e.method.toLowerCase().includes(q) ||
        (e.operation.summary || '').toLowerCase().includes(q) ||
        e.tag.toLowerCase().includes(q),
    );
  }, [endpoints, query]);

  const groupedEndpoints = useMemo(() => {
    const map = new Map<string, EndpointEntry[]>();
    for (const e of filteredEndpoints) {
      const list = map.get(e.tag) || [];
      list.push(e);
      map.set(e.tag, list);
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [filteredEndpoints]);

  const selected = useMemo(
    () => endpoints.find((e) => e.key === selectedKey) || null,
    [endpoints, selectedKey],
  );

  // When a new endpoint is chosen, initialize forms based on parameters.
  useEffect(() => {
    if (!selected || !spec) return;

    const pathInit: Record<string, string> = {};
    const queryInit: Record<string, string> = {};
    let bodyInit: unknown = null;

    for (const p of selected.operation.parameters || []) {
      if (p.in === 'path') pathInit[p.name] = '';
      else if (p.in === 'query') queryInit[p.name] = '';
      else if (p.in === 'body' && p.schema?.$ref) {
        bodyInit = buildExampleFromDefinition(spec, p.schema.$ref);
      }
    }

    setPathParams(pathInit);
    setQueryParams(queryInit);
    setBodyText(bodyInit ? JSON.stringify(bodyInit, null, 2) : '');
    setResult(null);
    setResultError(null);
  }, [selected, spec]);

  // Auto-select first connected instance when switching to instance mode
  useEffect(() => {
    if (apikeyMode !== 'instance') return;
    if (selectedInstanceId) return;
    const connected = instances.find((i) => i.status === 'open');
    if (connected?.apikey) {
      setSelectedInstanceId(connected.apikey);
    }
  }, [apikeyMode, instances, selectedInstanceId]);

  const resolvedApikey = useMemo(() => {
    if (apikeyMode === 'global') return globalApiKey || '';
    if (apikeyMode === 'custom') return customApikey;
    return selectedInstanceId; // already the instance token
  }, [apikeyMode, globalApiKey, customApikey, selectedInstanceId]);

  const pathHasParam = (name: string) =>
    selected && selected.path.includes(`{${name}}`);

  const buildFinalPath = useCallback(() => {
    if (!selected) return '';
    let p = selected.path;
    for (const [k, v] of Object.entries(pathParams)) {
      p = p.replace(`{${k}}`, encodeURIComponent(v));
    }
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(queryParams)) {
      if (v !== '') qs.append(k, v);
    }
    const qsStr = qs.toString();
    return qsStr ? `${p}?${qsStr}` : p;
  }, [selected, pathParams, queryParams]);

  const parseCustomHeaders = (raw: string): Record<string, string> => {
    const out: Record<string, string> = {};
    if (!raw.trim()) return out;
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        for (const [k, v] of Object.entries(parsed)) {
          out[k] = String(v);
        }
      }
    } catch {
      // fallback: linhas "Key: value"
      for (const line of raw.split('\n')) {
        const idx = line.indexOf(':');
        if (idx > 0) {
          const k = line.slice(0, idx).trim();
          const v = line.slice(idx + 1).trim();
          if (k) out[k] = v;
        }
      }
    }
    return out;
  };

  const handleExecute = async () => {
    if (!selected) return;
    if (!apiUrl) {
      toast.error('apiUrl nao definida no store');
      return;
    }

    setExecuting(true);
    setResult(null);
    setResultError(null);

    const base = apiUrl.replace(/\/$/, '');
    const finalPath = buildFinalPath();
    const url = `${base}${finalPath}`;
    const headers: Record<string, string> = {
      ...parseCustomHeaders(customHeaders),
    };
    if (resolvedApikey) headers['apikey'] = resolvedApikey;

    let bodyParsed: unknown = undefined;
    const hasBody = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(
      selected.method,
    );
    if (hasBody && bodyText.trim()) {
      try {
        bodyParsed = JSON.parse(bodyText);
        headers['Content-Type'] = headers['Content-Type'] || 'application/json';
      } catch {
        setResultError('Body nao e um JSON valido.');
        setExecuting(false);
        return;
      }
    }

    const started = performance.now();
    try {
      const res = await axios.request({
        url,
        method: selected.method as never,
        headers,
        data: bodyParsed,
        validateStatus: () => true, // treat all status as success
        transformRequest: (d) => (d === undefined ? undefined : JSON.stringify(d)),
      });

      const durationMs = Math.round(performance.now() - started);

      const hdrs: Record<string, string> = {};
      const rawHeaders = res.headers;
      if (rawHeaders instanceof AxiosHeaders) {
        rawHeaders.forEach((val: unknown, key: string) => {
          hdrs[key] = String(val);
        });
      } else if (rawHeaders && typeof rawHeaders === 'object') {
        for (const [k, v] of Object.entries(rawHeaders)) {
          hdrs[k] = String(v);
        }
      }

      setResult({
        status: res.status,
        statusText: res.statusText || '',
        durationMs,
        data: res.data,
        headers: hdrs,
        finalUrl: url,
        request: {
          method: selected.method,
          url,
          headers,
          body: bodyParsed,
        },
      });
    } catch (err: unknown) {
      const durationMs = Math.round(performance.now() - started);
      const msg =
        (err as { message?: string })?.message || 'Erro de rede desconhecido';
      setResultError(`${msg} (em ${durationMs}ms)`);
    } finally {
      setExecuting(false);
    }
  };

  return (
    <div className="flex h-[calc(100vh-4rem)] overflow-hidden">
      {/* Left: endpoint list */}
      <aside className="flex w-80 flex-col border-r border-sidebar-border bg-sidebar/60">
        <div className="border-b border-sidebar-border p-3">
          <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
            <Code2 className="h-4 w-4 text-purple-500" />
            Endpoints
            <button
              type="button"
              onClick={loadSpec}
              className="ml-auto rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
              title="Recarregar swagger"
            >
              <RefreshCcw className="h-3.5 w-3.5" />
            </button>
          </h2>
          <div className="relative">
            <SearchIcon className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              placeholder="Filtrar por path, metodo, tag..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-full rounded-md border border-input bg-background py-1.5 pl-8 pr-2 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {loadingSpec && (
            <div className="p-4 text-center text-sm text-muted-foreground">
              <Loader2 className="mx-auto mb-2 h-5 w-5 animate-spin" />
              Carregando swagger...
            </div>
          )}
          {specError && (
            <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
              {specError}
            </div>
          )}
          {!loadingSpec && !specError && groupedEndpoints.length === 0 && (
            <p className="p-4 text-center text-xs text-muted-foreground">
              Nenhum endpoint encontrado.
            </p>
          )}

          {groupedEndpoints.map(([tag, list]) => (
            <div key={tag} className="mb-3">
              <p className="mb-1 px-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                {tag}
              </p>
              <div className="space-y-0.5">
                {list.map((ep) => {
                  const color = METHOD_COLOR[ep.method.toLowerCase()] || '';
                  const active = ep.key === selectedKey;
                  return (
                    <button
                      key={ep.key}
                      onClick={() => setSelectedKey(ep.key)}
                      className={`group flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors ${
                        active
                          ? 'bg-primary/10 ring-1 ring-primary/40'
                          : 'hover:bg-accent'
                      }`}
                    >
                      <span
                        className={`inline-block w-12 shrink-0 rounded border px-1.5 py-0.5 text-center font-mono text-[10px] font-bold ${color}`}
                      >
                        {ep.method}
                      </span>
                      <span className="flex-1 truncate font-mono text-foreground">
                        {ep.path}
                      </span>
                      <ChevronRight className="h-3 w-3 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100" />
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </aside>

      {/* Right: request + response */}
      <section className="flex flex-1 flex-col overflow-hidden">
        {!selected ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-2 text-muted-foreground">
            <FileJson className="h-10 w-10" />
            <p className="text-sm">Selecione um endpoint a esquerda.</p>
          </div>
        ) : (
          <div className="flex flex-1 flex-col overflow-hidden">
            <div className="border-b border-sidebar-border bg-sidebar/30 px-5 py-3">
              <div className="flex items-center gap-3">
                <span
                  className={`rounded border px-2 py-0.5 font-mono text-xs font-bold ${
                    METHOD_COLOR[selected.method.toLowerCase()] || ''
                  }`}
                >
                  {selected.method}
                </span>
                <code className="flex-1 truncate font-mono text-sm text-foreground">
                  {selected.path}
                </code>
                <button
                  type="button"
                  onClick={handleExecute}
                  disabled={executing}
                  className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                >
                  {executing ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      Executando...
                    </>
                  ) : (
                    <>
                      <Play className="h-3.5 w-3.5" />
                      Executar
                    </>
                  )}
                </button>
              </div>
              {selected.operation.summary && (
                <p className="mt-2 text-xs text-foreground/80">
                  {selected.operation.summary}
                </p>
              )}
            </div>

            <div className="grid flex-1 grid-cols-1 gap-0 overflow-hidden lg:grid-cols-2">
              {/* Request panel */}
              <div className="overflow-y-auto border-r border-sidebar-border p-5">
                <div className="space-y-4">
                  {/* apikey */}
                  <div className="rounded-md border border-sidebar-border p-3">
                    <label className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-foreground">
                      <KeyRound className="h-3.5 w-3.5 text-amber-500" />
                      Autenticacao (header apikey)
                    </label>
                    <div className="mb-2 flex gap-2">
                      {(['global', 'instance', 'custom'] as ApikeyMode[]).map(
                        (m) => (
                          <label
                            key={m}
                            className={`flex-1 cursor-pointer rounded-md border px-2 py-1.5 text-center text-xs ${
                              apikeyMode === m
                                ? 'border-primary bg-primary/10 text-foreground'
                                : 'border-sidebar-border text-muted-foreground hover:bg-accent'
                            }`}
                          >
                            <input
                              type="radio"
                              name="apikeyMode"
                              value={m}
                              checked={apikeyMode === m}
                              onChange={() => setApikeyMode(m)}
                              className="mr-1"
                            />
                            {m === 'global'
                              ? 'Global'
                              : m === 'instance'
                                ? 'Instancia'
                                : 'Custom'}
                          </label>
                        ),
                      )}
                    </div>

                    {apikeyMode === 'instance' && (
                      <select
                        value={selectedInstanceId}
                        onChange={(e) => setSelectedInstanceId(e.target.value)}
                        className="w-full rounded-md border border-input bg-background px-2 py-1.5 text-xs text-foreground"
                      >
                        <option value="">-- selecione uma instancia --</option>
                        {instances.map((inst) => (
                          <option key={inst.instanceName} value={inst.apikey}>
                            {inst.instanceName}{' '}
                            {inst.status === 'open' ? '(conectada)' : '(off)'}
                          </option>
                        ))}
                      </select>
                    )}
                    {apikeyMode === 'custom' && (
                      <input
                        type="text"
                        placeholder="Cole a apikey aqui..."
                        value={customApikey}
                        onChange={(e) => setCustomApikey(e.target.value)}
                        className="w-full rounded-md border border-input bg-background px-2 py-1.5 font-mono text-xs text-foreground placeholder:text-muted-foreground"
                      />
                    )}
                    {apikeyMode === 'global' && (
                      <p className="font-mono text-[11px] text-muted-foreground">
                        {globalApiKey
                          ? `${globalApiKey.slice(0, 6)}...${globalApiKey.slice(-4)}`
                          : '(nenhuma no store)'}
                      </p>
                    )}
                  </div>

                  {/* Path params */}
                  {Object.keys(pathParams).length > 0 && (
                    <div>
                      <h3 className="mb-1.5 text-xs font-semibold text-foreground">
                        Path parameters
                      </h3>
                      <div className="space-y-1.5">
                        {Object.keys(pathParams).map((name) => (
                          <div key={name} className="flex items-center gap-2">
                            <span className="w-24 shrink-0 font-mono text-xs text-muted-foreground">
                              {name}
                              {pathHasParam(name) ? '*' : ''}
                            </span>
                            <input
                              type="text"
                              value={pathParams[name]}
                              onChange={(e) =>
                                setPathParams({
                                  ...pathParams,
                                  [name]: e.target.value,
                                })
                              }
                              className="flex-1 rounded-md border border-input bg-background px-2 py-1.5 text-xs text-foreground"
                            />
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Query params */}
                  {Object.keys(queryParams).length > 0 && (
                    <div>
                      <h3 className="mb-1.5 text-xs font-semibold text-foreground">
                        Query parameters
                      </h3>
                      <div className="space-y-1.5">
                        {Object.keys(queryParams).map((name) => (
                          <div key={name} className="flex items-center gap-2">
                            <span className="w-24 shrink-0 font-mono text-xs text-muted-foreground">
                              {name}
                            </span>
                            <input
                              type="text"
                              value={queryParams[name]}
                              onChange={(e) =>
                                setQueryParams({
                                  ...queryParams,
                                  [name]: e.target.value,
                                })
                              }
                              className="flex-1 rounded-md border border-input bg-background px-2 py-1.5 text-xs text-foreground"
                            />
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Body */}
                  {['POST', 'PUT', 'PATCH', 'DELETE'].includes(
                    selected.method,
                  ) && (
                    <div>
                      <div className="mb-1.5 flex items-center justify-between">
                        <h3 className="text-xs font-semibold text-foreground">
                          Body (JSON)
                        </h3>
                        <button
                          type="button"
                          onClick={() => {
                            try {
                              const parsed = JSON.parse(bodyText);
                              setBodyText(JSON.stringify(parsed, null, 2));
                            } catch {
                              toast.error('Body atual nao e um JSON valido.');
                            }
                          }}
                          className="rounded px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-accent hover:text-foreground"
                        >
                          Formatar
                        </button>
                      </div>
                      <textarea
                        rows={14}
                        value={bodyText}
                        spellCheck={false}
                        onChange={(e) => setBodyText(e.target.value)}
                        className="w-full rounded-md border border-input bg-background px-2 py-1.5 font-mono text-[11px] leading-relaxed text-foreground"
                      />
                    </div>
                  )}

                  {/* Custom headers */}
                  <div>
                    <h3 className="mb-1.5 text-xs font-semibold text-foreground">
                      Headers extras (JSON ou Key: value por linha)
                    </h3>
                    <textarea
                      rows={4}
                      spellCheck={false}
                      placeholder='{"X-Custom-Header": "valor"}'
                      value={customHeaders}
                      onChange={(e) => setCustomHeaders(e.target.value)}
                      className="w-full rounded-md border border-input bg-background px-2 py-1.5 font-mono text-[11px] text-foreground placeholder:text-muted-foreground"
                    />
                  </div>

                  {/* Description */}
                  {selected.operation.description && (
                    <div className="rounded-md border border-sidebar-border bg-sidebar/40 p-3">
                      <h3 className="mb-1 text-xs font-semibold text-foreground">
                        Descricao
                      </h3>
                      <pre className="whitespace-pre-wrap text-[11px] leading-relaxed text-foreground/85">
                        {selected.operation.description}
                      </pre>
                    </div>
                  )}
                </div>
              </div>

              {/* Response panel */}
              <div className="overflow-y-auto p-5">
                <h3 className="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
                  <FileJson className="h-4 w-4 text-blue-500" />
                  Resposta
                </h3>
                {resultError && (
                  <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
                    {resultError}
                  </div>
                )}
                {!result && !resultError && (
                  <p className="text-xs text-muted-foreground">
                    Clique em "Executar" para ver a resposta aqui.
                  </p>
                )}
                {result && (
                  <div className="space-y-3">
                    <div className="flex flex-wrap items-center gap-2 text-xs">
                      <span
                        className={`rounded px-2 py-0.5 font-mono font-bold ${
                          result.status >= 200 && result.status < 300
                            ? 'bg-emerald-500/15 text-emerald-500'
                            : result.status >= 400
                              ? 'bg-red-500/15 text-red-500'
                              : 'bg-amber-500/15 text-amber-500'
                        }`}
                      >
                        {result.status} {result.statusText}
                      </span>
                      <span className="inline-flex items-center gap-1 text-muted-foreground">
                        <Clock className="h-3 w-3" />
                        {result.durationMs} ms
                      </span>
                      <span className="truncate font-mono text-muted-foreground">
                        {result.finalUrl}
                      </span>
                    </div>

                    <div>
                      <p className="mb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        Request
                      </p>
                      <pre className="max-h-48 overflow-auto rounded-md border border-sidebar-border bg-sidebar/40 p-2 font-mono text-[11px] leading-relaxed text-foreground">
                        {pretty({
                          method: result.request.method,
                          url: result.request.url,
                          headers: result.request.headers,
                          body: result.request.body,
                        })}
                      </pre>
                    </div>

                    <div>
                      <p className="mb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        Response headers
                      </p>
                      <pre className="max-h-32 overflow-auto rounded-md border border-sidebar-border bg-sidebar/40 p-2 font-mono text-[11px] leading-relaxed text-foreground">
                        {pretty(result.headers)}
                      </pre>
                    </div>

                    <div>
                      <p className="mb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        Response body
                      </p>
                      <pre className="max-h-[40vh] overflow-auto rounded-md border border-sidebar-border bg-sidebar/40 p-2 font-mono text-[11px] leading-relaxed text-foreground">
                        {pretty(result.data)}
                      </pre>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

export default ApiTester;
