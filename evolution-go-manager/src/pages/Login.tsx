import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Button,
  Input,
  Label,
  Alert,
  AlertTitle,
  AlertDescription,
} from '@evoapi/design-system';
import { toast } from 'sonner';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertCircle } from 'lucide-react';

import useAuth from '@/hooks/useAuth';
import { initRegister } from '@/services/api/license';

export const Login: React.FC = () => {
  const { login, checkLicense, setApiUrl, setApiKey: setStoreApiKey, isAuthenticated, licenseState, apiUrl: defaultApiUrl, apiKey: storedApiKey } = useAuth();
  const navigate = useNavigate();

  const [isLoading, setIsLoading] = useState(false);
  const [loginError, setLoginError] = useState('');

  // If user is already authenticated and licensed, redirect to dashboard
  useEffect(() => {
    if (isAuthenticated && licenseState === 'licensed') {
      navigate('/manager', { replace: true });
    }
  }, [isAuthenticated, licenseState, navigate]);
  
  // Usar a URL atual do navegador como placeholder
  const currentUrl = window.location.origin;

  // Schema de validação para login
const loginSchema = z.object({
  apiUrl: z
    .string()
      .min(1, { message: 'URL da API é obrigatória' })
      .url({ message: `URL inválida. Use o formato: ${currentUrl}` })
    .refine((url) => url.startsWith('http://') || url.startsWith('https://'), {
      message: 'URL deve começar com http:// ou https://',
    }),
  apiKey: z
    .string()
      .min(1, { message: 'API Key é obrigatória' })
      .min(10, { message: 'API Key deve ter pelo menos 10 caracteres' }),
});

type LoginFormData = z.infer<typeof loginSchema>;

  // Formulário de login
  const loginForm = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      apiUrl: defaultApiUrl,
      apiKey: storedApiKey || '',
    },
  });

  const onLoginSubmit = async (data: LoginFormData) => {
    setIsLoading(true);
    setLoginError('');

    try {
      // 1. Check license FIRST (before connecting to backend)
      const cleanUrl = data.apiUrl.replace(/\/$/, '');
      toast.info('Verificando licenca...');
      const licResult = await checkLicense(cleanUrl, data.apiKey);

      if (licResult !== 'licensed') {
        // License not found - initiate registration (no need to connect to backend)
        toast.info('Licenca necessaria', {
          description: 'Redirecionando para registro de licenca...',
        });

        const callbackUrl = `${window.location.origin}/manager/license/callback`;
        const registerData = await initRegister(callbackUrl, cleanUrl, data.apiKey);

        if (!registerData.register_url) {
          toast.error('Erro', {
            description: registerData.message || 'Falha ao iniciar registro de licenca.',
          });
          setLoginError(registerData.message || 'Falha ao iniciar registro.');
          return;
        }

        // Save credentials so callback page knows where to call activate
        setApiUrl(cleanUrl);
        setStoreApiKey(data.apiKey);

        // Redirect to licensing registration page
        window.location.href = registerData.register_url;
        return;
      }

      // 2. License OK - now connect to backend
      await login(data.apiUrl, data.apiKey);

      toast.success('Conectado com sucesso!', {
        description: 'Licenca valida. Bem-vindo!',
      });

      navigate('/manager', { replace: true });
    } catch (error) {
      console.error('Erro ao fazer login:', error);
      const errorMessage =
        error instanceof Error
          ? error.message
          : 'Erro ao conectar. Verifique a URL e API Key.';

      toast.error('Erro', {
        description: errorMessage,
      });

      setLoginError(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-gradient-to-t from-primary/20 via-background/95 to-background relative">
      <div className="w-full max-w-md space-y-6">
        {/* Logo */}
        <div className="text-center">
          <h1 className="text-3xl font-bold text-primary">
            Evolution GO
          </h1>
        </div>

        {/* Formulário */}
        <div className="bg-background/80 backdrop-blur-sm border rounded-lg p-6 shadow-lg">
          <div className="space-y-2 mb-6">
            <h2 className="text-2xl font-bold">Entrar na sua conta</h2>
            <p className="text-muted-foreground">Digite suas credenciais para acessar o sistema</p>
          </div>

          {/* Mostrar mensagem de erro da aba login */}
          {loginError && (
            <Alert variant="destructive" className="mb-4">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Erro</AlertTitle>
              <AlertDescription>{loginError}</AlertDescription>
            </Alert>
          )}

          <form onSubmit={loginForm.handleSubmit(onLoginSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="login-apiUrl">URL da API Evolution GO</Label>
              <Input
                id="login-apiUrl"
                type="text"
                placeholder={currentUrl}
                disabled={isLoading}
                {...loginForm.register('apiUrl')}
              />
              {loginForm.formState.errors.apiUrl && (
                <p className="text-destructive text-sm">
                  {loginForm.formState.errors.apiUrl.message}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="login-apiKey">API Key (GLOBAL_API_KEY)</Label>
              <Input
                id="login-apiKey"
                type="password"
                placeholder="Sua chave de API"
                disabled={isLoading}
                {...loginForm.register('apiKey')}
              />
              {loginForm.formState.errors.apiKey && (
                <p className="text-destructive text-sm">
                  {loginForm.formState.errors.apiKey.message}
                </p>
              )}
            </div>

            <div className="text-xs text-muted-foreground">
              <p>
                <strong>Dica:</strong> A API Key é o valor da variável{' '}
                <code className="relative rounded bg-muted px-[0.3rem] py-[0.2rem] font-mono text-xs">
                  GLOBAL_API_KEY
                </code>{' '}
                configurada no arquivo .env do Evolution GO.
              </p>
            </div>

            <Button type="submit" disabled={isLoading} className="w-full">
              {isLoading ? 'Conectando...' : 'Entrar'}
            </Button>
          </form>
        </div>

        {/* Mensagem de termos de serviço */}
        <div className="text-center text-xs text-muted-foreground">
          <p>
            Ao continuar, você concorda com nossos{' '}
            <a href="#" className="underline hover:text-primary">
              Termos de Serviço
            </a>{' '}
            e{' '}
            <a href="#" className="underline hover:text-primary">
              Política de Privacidade
            </a>
            .
          </p>
        </div>
      </div>
    </div>
  );
};

export default Login;
