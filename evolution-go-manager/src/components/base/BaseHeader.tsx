/**
 * Base Header Component
 * Reusable header with search, actions, and filters
 * Based on evo-ai-frontend BaseHeader
 */

import { ReactNode } from 'react';
import {
  Button,
  Input,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Badge,
} from '@evoapi/design-system';
import { Search, Filter, MoreVertical, X } from 'lucide-react';

export interface HeaderAction {
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  variant?: 'default' | 'outline' | 'ghost' | 'destructive';
  show?: boolean;
  className?: string;
}

export interface HeaderFilter {
  label: string;
  value: string | number;
  onRemove: () => void;
}

export interface BaseHeaderProps {
  title: string;
  subtitle?: string;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  searchPlaceholder?: string;
  primaryAction?: HeaderAction;
  secondaryActions?: HeaderAction[];
  moreActions?: HeaderAction[];
  filters?: HeaderFilter[];
  onFilterClick?: () => void;
  showFilters?: boolean;
  totalCount?: number;
  selectedCount?: number;
  onClearSelection?: () => void;
  bulkActions?: HeaderAction[];
  className?: string;
  children?: ReactNode;
}

export default function BaseHeader({
  title,
  subtitle,
  searchValue,
  onSearchChange,
  searchPlaceholder = 'Buscar...',
  primaryAction,
  secondaryActions = [],
  moreActions = [],
  filters = [],
  onFilterClick,
  showFilters = false,
  selectedCount = 0,
  onClearSelection,
  bulkActions = [],
  className = '',
  children,
}: BaseHeaderProps) {
  const hasSelection = selectedCount > 0;
  const visibleSecondaryActions = secondaryActions.filter(
    (action) => action.show !== false
  );
  const visibleMoreActions = moreActions.filter(
    (action) => action.show !== false
  );

  return (
    <div className={`space-y-6 ${className} mb-4`}>
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        {/* Title Section */}
        <div className="flex-1">
          <h1 className="text-2xl font-bold tracking-tight text-sidebar-foreground mb-2 dark:text-gray-200">
            {title}
          </h1>
          {subtitle && (
            <p className="text-sm text-sidebar-foreground/70 dark:text-gray-400">{subtitle}</p>
          )}
        </div>

        {/* Primary Action */}
        {primaryAction && primaryAction.show !== false && (
          <div className="flex-shrink-0">
            <Button
              onClick={primaryAction.onClick}
              variant={primaryAction.variant || 'default'}
              className={primaryAction.className}
            >
              {primaryAction.icon && (
                <span className="mr-2">{primaryAction.icon}</span>
              )}
              {primaryAction.label}
            </Button>
          </div>
        )}
      </div>

      {/* Search and Filter Row */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3 flex-1">
          {/* Search */}
          {onSearchChange && (
            <div className="relative flex-1 max-w-md">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-sidebar-foreground/60 dark:text-gray-200" />
              <Input
                type="search"
                placeholder={searchPlaceholder}
                value={searchValue}
                onChange={(e) => onSearchChange(e.target.value)}
                className="pl-9 bg-sidebar border-sidebar-border text-sidebar-foreground placeholder:text-sidebar-foreground/50 focus:border-sidebar-border dark:text-gray-400 dark:placeholder:text-gray-400"
              />
            </div>
          )}

          {/* Filter Button */}
          {showFilters && onFilterClick && (
            <Button
              variant="outline"
              size="sm"
              onClick={onFilterClick}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent whitespace-nowrap dark:text-gray-400 dark:hover:bg-sidebar-accent"
            >
              <Filter className="h-4 w-4 mr-2" />
              Filtros
              {filters.length > 0 && (
                <Badge
                  variant="secondary"
                  className="ml-2 h-5 px-1.5 text-xs bg-sidebar-accent"
                >
                  {filters.length}
                </Badge>
              )}
            </Button>
          )}
        </div>

        {/* Secondary Actions */}
        <div className="flex items-center gap-2">
          {visibleSecondaryActions.map((action, index) => (
            <Button
              key={index}
              variant={action.variant || 'outline'}
              size="sm"
              onClick={action.onClick}
              className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:bg-sidebar-accent"
            >
              {action.icon && <span className="mr-2">{action.icon}</span>}
              {action.label}
            </Button>
          ))}

          {/* More Actions Dropdown */}
          {visibleMoreActions.length > 0 && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="bg-sidebar border-sidebar-border text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:bg-sidebar-accent"
                >
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="end"
                className="bg-sidebar border-sidebar-border text-sidebar-foreground dark:text-gray-400"
              >
                {visibleMoreActions.map((action, index) => (
                  <DropdownMenuItem
                    key={index}
                    onClick={action.onClick}
                    className={`hover:bg-sidebar-accent ${action.variant === 'destructive' ? 'text-red-400' : ''} dark:text-gray-400`}
                  >
                    {action.icon && <span className="mr-2">{action.icon}</span>}
                    {action.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>

      {/* Selection Bar */}
      {hasSelection && (
        <div className="flex items-center justify-between rounded-lg bg-sidebar-accent/50 border border-sidebar-border px-4 py-2">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-sidebar-foreground dark:text-gray-400">
              {selectedCount} selecionado{selectedCount > 1 ? 's' : ''}
            </span>
            {onClearSelection && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onClearSelection}
                className="h-7 px-2 text-sidebar-foreground/70 hover:text-sidebar-foreground hover:bg-sidebar-accent dark:text-gray-400 dark:hover:text-gray-400 dark:hover:bg-sidebar-accent"
              >
                <X className="h-3 w-3 mr-1" />
                Limpar
              </Button>
            )}
          </div>
          {bulkActions.length > 0 && (
            <div className="flex items-center gap-2">
              {bulkActions.map((action, index) => (
                <Button
                  key={index}
                  variant={action.variant || 'outline'}
                  size="sm"
                  onClick={action.onClick}
                  className="h-7 bg-sidebar-accent border-sidebar-border text-sidebar-foreground hover:bg-sidebar dark:text-gray-400 dark:hover:bg-sidebar"
                >
                  {action.icon && <span className="mr-1.5">{action.icon}</span>}
                  {action.label}
                </Button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Active Filters */}
      {filters.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {filters.map((filter, index) => (
            <Badge
              key={index}
              variant="secondary"
              className="pl-2 pr-1 py-1 bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar dark:text-gray-400 dark:hover:bg-sidebar dark:bg-sidebar-accent/50"
            >
              {filter.label}: {filter.value}
              <Button
                variant="ghost"
                size="sm"
                onClick={filter.onRemove}
                className="ml-1 h-4 w-4 p-0 hover:bg-transparent text-sidebar-foreground/60 hover:text-sidebar-foreground dark:text-gray-400 dark:hover:text-gray-400"
              >
                <X className="h-3 w-3" />
              </Button>
            </Badge>
          ))}
        </div>
      )}

      {/* Custom Content */}
      {children}
    </div>
  );
}
