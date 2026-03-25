import React, { useEffect, useState, useCallback } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Button } from '@evoapi/design-system';
import { toast } from 'sonner';
import { Loader2, CheckCircle, XCircle } from 'lucide-react';

import { activateLicense } from '@/services/api/license';
import useAuth from '@/hooks/useAuth';

type CallbackState = 'activating' | 'success' | 'error';

const LicenseCallback: React.FC = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { setLicenseState, apiUrl, apiKey } = useAuth();

  const [state, setState] = useState<CallbackState>('activating');
  const [errorMessage, setErrorMessage] = useState('');

  const code = searchParams.get('code');

  const doActivate = useCallback(async () => {
    if (!code) {
      setState('error');
      setErrorMessage('Codigo de autorizacao nao encontrado na URL.');
      return;
    }

    setState('activating');
    setErrorMessage('');

    try {
      const result = await activateLicense(code, apiUrl, apiKey);

      if (result.status === 'active') {
        setState('success');
        setLicenseState('licensed');
        toast.success('Licenca ativada com sucesso!');

        setTimeout(() => {
          navigate('/manager/login', { replace: true });
        }, 2000);
      } else {
        setState('error');
        setErrorMessage(result.message || 'Falha ao ativar licenca.');
      }
    } catch (err: unknown) {
      setState('error');
      const msg = err instanceof Error ? err.message : (err as { message?: string })?.message;
      setErrorMessage(msg || 'Erro ao ativar licenca.');
    }
  }, [code, apiUrl, apiKey, navigate, setLicenseState]);

  useEffect(() => {
    doActivate();
  }, [doActivate]);

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-gradient-to-t from-primary/20 via-background/95 to-background">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-primary">Evolution GO</h1>
        </div>

        <div className="bg-background/80 backdrop-blur-sm border rounded-lg p-8 shadow-lg text-center space-y-4">
          {state === 'activating' && (
            <>
              <Loader2 className="h-12 w-12 animate-spin text-primary mx-auto" />
              <h2 className="text-xl font-semibold">Ativando licenca...</h2>
              <p className="text-muted-foreground">
                Aguarde enquanto ativamos sua licenca.
              </p>
            </>
          )}

          {state === 'success' && (
            <>
              <CheckCircle className="h-12 w-12 text-green-500 mx-auto" />
              <h2 className="text-xl font-semibold">Licenca ativada!</h2>
              <p className="text-muted-foreground">
                Redirecionando para o login...
              </p>
            </>
          )}

          {state === 'error' && (
            <>
              <XCircle className="h-12 w-12 text-destructive mx-auto" />
              <h2 className="text-xl font-semibold">Erro na ativacao</h2>
              <p className="text-muted-foreground">{errorMessage}</p>
              <div className="flex gap-2 justify-center pt-2">
                <Button
                  variant="outline"
                  onClick={() => navigate('/manager/login', { replace: true })}
                >
                  Voltar ao login
                </Button>
                <Button onClick={doActivate}>
                  Tentar novamente
                </Button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default LicenseCallback;
