import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from 'sonner';
import ErrorBoundary from '@/components/base/ErrorBoundary';
import Layout from '@/components/base/Layout';
import Home from '@/pages/Home';
import Login from '@/pages/Login';
import Dashboard from '@/pages/Dashboard';
import Instances from '@/pages/Instances';
import InstanceSettings from '@/pages/InstanceSettings';
import Messages from '@/pages/Messages';
import Events from '@/pages/Events';
import Settings from '@/pages/Settings';
import LicenseCallback from '@/pages/LicenseCallback';
import ApiTester from '@/pages/ApiTester';
import useAuth from '@/hooks/useAuth';
import { DarkModeProvider } from '@/contexts/ThemeContext';

function App() {
  const { isAuthenticated, licenseState } = useAuth();

  // User must be authenticated AND have a valid license to access protected routes
  const isFullyAuthorized = isAuthenticated && licenseState === 'licensed';

  return (
    <DarkModeProvider>
      <ErrorBoundary>
        <BrowserRouter>
          <Routes>
            {/* Landing Page - Public */}
            <Route path="/" element={<Home />} />

            {/* Manager Login - Public */}
            <Route
              path="/manager/login"
              element={!isFullyAuthorized ? <Login /> : <Navigate to="/manager" replace />}
            />

            {/* License Callback - Public (must be before the wildcard catch) */}
            <Route path="/manager/license/callback" element={<LicenseCallback />} />

            {/* Manager Protected Routes - requires auth + valid license */}
            {isFullyAuthorized ? (
              <Route path="/manager" element={<Layout />}>
              <Route index element={<Dashboard />} />
              <Route path="instances" element={<Instances />} />
              <Route path="instances/:instanceId/settings" element={<InstanceSettings />} />
              <Route path="messages" element={<Messages />} />
              <Route path="events" element={<Events />} />
              <Route path="api-tester" element={<ApiTester />} />
              <Route path="settings" element={<Settings />} />
            </Route>
            ) : (
              /* Redirect to login if not authenticated or not licensed */
              <Route path="/manager/*" element={<Navigate to="/manager/login" replace />} />
            )}

            {/* Catch all - redirect to home */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
        <Toaster position="top-right" richColors />
      </ErrorBoundary>
    </DarkModeProvider>
  );
}

export default App;
