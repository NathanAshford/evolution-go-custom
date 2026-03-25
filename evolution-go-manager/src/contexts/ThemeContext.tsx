import React, { createContext, useEffect, useState, useMemo } from 'react';

export type Theme = 'light' | 'dark';

export interface DarkModeContextType {
  theme: Theme;
  toggleTheme: () => void;
}

// Create context for theme
export const DarkModeContext = createContext<DarkModeContextType | undefined>(undefined);

// Provider that will be used at the root of the application
export function DarkModeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    // If we're on the server, return 'light' as default
    if (typeof window === 'undefined') return 'light';

    // Check localStorage
    const savedTheme = localStorage.getItem('evolution-theme') as Theme;
    if (savedTheme) return savedTheme;

    // Check system preference
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }

    return 'light';
  });

  // Apply theme to document
  useEffect(() => {
    const root = window.document.documentElement;

    if (theme === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }

    // Save to localStorage
    localStorage.setItem('evolution-theme', theme);
  }, [theme]);

  // Watch for changes in system preference
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    const handleChange = () => {
      const savedTheme = localStorage.getItem('evolution-theme') as Theme;
      if (!savedTheme) {
        setTheme(mediaQuery.matches ? 'dark' : 'light');
      }
    };

    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, []);

  const toggleTheme = () => {
    setTheme(prevTheme => (prevTheme === 'light' ? 'dark' : 'light'));
  };

  // Memoize context value to avoid unnecessary re-renders
  const contextValue = useMemo(() => ({ theme, toggleTheme }), [theme]);

  return <DarkModeContext.Provider value={contextValue}>{children}</DarkModeContext.Provider>;
}
