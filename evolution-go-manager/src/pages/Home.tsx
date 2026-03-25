import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@evoapi/design-system';
import { ArrowRight, Zap, Shield, Globe } from 'lucide-react';

export const Home: React.FC = () => {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary/10 via-background to-background">
      {/* Hero Section */}
      <div className="container mx-auto px-4 py-16">
        <div className="max-w-4xl mx-auto text-center space-y-8">
          {/* Logo/Title */}
          <div className="space-y-4">
            <h1 className="text-6xl font-bold text-primary animate-fadeIn">
              Evolution GO
            </h1>
            <p className="text-xl text-muted-foreground">
              Interface moderna para gerenciamento de instâncias WhatsApp
            </p>
          </div>

          {/* Features */}
          <div className="grid md:grid-cols-3 gap-6 mt-12">
            <div className="bg-card border rounded-lg p-6 space-y-3 hover:border-primary transition-colors">
              <div className="w-12 h-12 bg-primary/10 rounded-lg flex items-center justify-center">
                <Zap className="w-6 h-6 text-primary" />
              </div>
              <h3 className="text-lg font-semibold">Rápido e Eficiente</h3>
              <p className="text-sm text-muted-foreground">
                Gerencie múltiplas instâncias WhatsApp com alta performance
              </p>
            </div>

            <div className="bg-card border rounded-lg p-6 space-y-3 hover:border-primary transition-colors">
              <div className="w-12 h-12 bg-primary/10 rounded-lg flex items-center justify-center">
                <Shield className="w-6 h-6 text-primary" />
              </div>
              <h3 className="text-lg font-semibold">Seguro</h3>
              <p className="text-sm text-muted-foreground">
                Autenticação robusta e controle total sobre suas instâncias
              </p>
            </div>

            <div className="bg-card border rounded-lg p-6 space-y-3 hover:border-primary transition-colors">
              <div className="w-12 h-12 bg-primary/10 rounded-lg flex items-center justify-center">
                <Globe className="w-6 h-6 text-primary" />
              </div>
              <h3 className="text-lg font-semibold">API Completa</h3>
              <p className="text-sm text-muted-foreground">
                Integração via API REST com Evolution GO
              </p>
            </div>
          </div>

          {/* CTA Button */}
          <div className="mt-12">
            <Button
              size="lg"
              onClick={() => navigate('/manager/login')}
              className="text-lg px-8 py-6 group"
            >
              Acessar Manager
              <ArrowRight className="ml-2 w-5 h-5 group-hover:translate-x-1 transition-transform" />
            </Button>
          </div>

          {/* Footer Info */}
          <div className="mt-16 pt-8 border-t">
            <p className="text-sm text-muted-foreground">
              Evolution GO Manager - Gerencie suas instâncias WhatsApp de forma simples e eficiente
            </p>
            <p className="text-xs text-muted-foreground mt-2">
              © {new Date().getFullYear()} Evolution GO. Todos os direitos reservados.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;

