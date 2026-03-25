import { useContext } from 'react';
import { DarkModeContext } from '../contexts/ThemeContext';

export function useDarkMode() {
  const context = useContext(DarkModeContext);
  if (context === undefined) {
    throw new Error('useDarkMode deve ser usado dentro de um DarkModeProvider');
  }
  return context;
}
