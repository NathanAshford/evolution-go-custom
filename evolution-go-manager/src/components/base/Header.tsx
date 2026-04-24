import { LogOut, Moon, Sun, TerminalSquare, BookOpen } from 'lucide-react';
import { NavLink } from 'react-router-dom';
import useAuth from '@/hooks/useAuth';
import { useDarkMode } from '@/hooks/useDarkMode';

function Header() {
  const { logout, apiUrl } = useAuth();
  const { theme, toggleTheme } = useDarkMode();

  const handleLogout = () => {
    logout();
  };

  const swaggerHref = apiUrl
    ? `${apiUrl.replace(/\/$/, '')}/swagger/index.html`
    : '/swagger/index.html';

  return (
    <header className="flex h-16 items-center justify-between border-b border-sidebar-border bg-sidebar px-0 py-3 shadow-sm">
      {/* Left side - empty space aligned with sidebar width */}
      <div className="w-56 flex items-center px-4">
        {/* Empty - logo is in sidebar */}
      </div>

      {/* Right side */}
      <div className="flex items-center gap-2 px-4">
        <NavLink
          to="/manager/api-tester"
          className={({ isActive }) =>
            `flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? 'bg-purple-500/15 text-purple-400'
                : 'text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-200 dark:hover:bg-sidebar-accent'
            }`
          }
          title="Testar endpoints da API interativamente"
        >
          <TerminalSquare className="h-4 w-4" />
          <span className="hidden sm:inline">API Tester</span>
        </NavLink>

        <a
          href={swaggerHref}
          target="_blank"
          rel="noreferrer noopener"
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-200 dark:hover:bg-sidebar-accent"
          title="Abrir Swagger em nova aba"
        >
          <BookOpen className="h-4 w-4" />
          <span className="hidden sm:inline">Swagger</span>
        </a>

        <button
          onClick={toggleTheme}
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-200 dark:hover:bg-sidebar-accent"
          title={theme === 'dark' ? 'Modo claro' : 'Modo escuro'}
        >
          {theme === 'dark' ? (
            <Sun className="h-4 w-4" />
          ) : (
            <Moon className="h-4 w-4" />
          )}
        </button>

        <button
          onClick={handleLogout}
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-200 dark:hover:bg-sidebar-accent"
        >
          <LogOut className="h-4 w-4" />
          Sair
        </button>
      </div>
    </header>
  );
}

export default Header;
