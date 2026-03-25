import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Smartphone,
} from 'lucide-react';
import { cn } from '@/utils/cn';

const navItems = [
  { to: '/manager', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/manager/instances', label: 'Instâncias', icon: Smartphone },
];

function Sidebar() {
  const currentYear = new Date().getFullYear();

  return (
    <div className="hidden md:flex bg-sidebar text-sidebar-foreground flex-col w-56 border-r border-sidebar-border">
      {/* Logo Header */}
      <div className="h-16 flex items-center px-4 border-b border-sidebar-border">
        <h2 className="text-lg font-bold text-primary">
          Evolution GO
        </h2>
      </div>

      {/* Navigation Menu */}
      <nav className="space-y-1.5 flex-1 px-2 py-4">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/manager'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 px-3 py-2.5 rounded-md transition-all',
                isActive
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              )
            }
          >
            {({ isActive }) => (
              <>
                <item.icon className={cn('flex-shrink-0 h-5 w-5', isActive && 'text-primary')} />
                <span className="font-medium">{item.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* Sidebar Footer */}
      <div className="mt-auto p-4 border-t border-sidebar-border">
        <div className="text-sm text-primary font-medium">Evolution GO</div>
        <div className="text-xs text-muted-foreground mt-1">
          © {currentYear} All rights reserved
        </div>
      </div>
    </div>
  );
}

export default Sidebar;
