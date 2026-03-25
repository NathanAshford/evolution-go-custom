/**
 * Empty State Component
 * Displays when there's no data to show
 */

import { Button } from '@evoapi/design-system';
import { LucideIcon } from 'lucide-react';
import { cn } from '@/utils/cn';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  className?: string;
}

export default function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center py-12 px-4 text-center',
        className
      )}
    >
      <div className="rounded-full bg-sidebar-accent/50 p-6 mb-4 dark:bg-sidebar-accent/50 dark:text-gray-400">
        <Icon className="h-12 w-12 text-sidebar-foreground/60 dark:text-gray-400" />
      </div>
      <h3 className="text-lg font-semibold text-sidebar-foreground mb-2 dark:text-gray-400">
        {title}
      </h3>
      <p className="text-sm text-sidebar-foreground/60 mb-6 max-w-md dark:text-gray-400">
        {description}
      </p>
      {action && (
        <Button onClick={action.onClick}>{action.label}</Button>
      )}
    </div>
  );
}
