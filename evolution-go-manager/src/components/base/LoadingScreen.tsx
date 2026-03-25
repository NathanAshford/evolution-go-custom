import { Loader2 } from 'lucide-react';

interface LoadingScreenProps {
  message?: string;
  fullScreen?: boolean;
}

function LoadingScreen({
  message = 'Carregando...',
  fullScreen = true
}: LoadingScreenProps) {
  const containerClass = fullScreen
    ? 'flex min-h-screen items-center justify-center bg-gray-50'
    : 'flex items-center justify-center p-8';

  return (
    <div className={containerClass}>
      <div className="text-center">
        <div className="mb-4 flex justify-center">
          <Loader2 className="h-12 w-12 animate-spin text-blue-600" />
        </div>
        <p className="text-sm font-medium text-gray-700">{message}</p>
      </div>
    </div>
  );
}

export default LoadingScreen;
