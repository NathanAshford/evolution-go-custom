/**
 * Instances Header Component
 * Header for instances page with search and actions
 */

import { Plus } from 'lucide-react';
import BaseHeader, { type HeaderAction } from '../base/BaseHeader';

interface InstancesHeaderProps {
  totalCount: number;
  selectedCount: number;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onNewInstance: () => void;
  onClearSelection: () => void;
}

export default function InstancesHeader({
  totalCount,
  selectedCount,
  searchValue,
  onSearchChange,
  onNewInstance,
  onClearSelection,
}: InstancesHeaderProps) {
  const primaryAction: HeaderAction = {
    label: 'Nova Instância',
    icon: <Plus className="h-4 w-4" />,
    onClick: onNewInstance,
  };

  return (
    <BaseHeader
      title="Instâncias"
      subtitle="Gerencie suas instâncias WhatsApp do Evolution GO"
      totalCount={totalCount}
      selectedCount={selectedCount}
      searchValue={searchValue}
      onSearchChange={onSearchChange}
      searchPlaceholder="Buscar instâncias..."
      primaryAction={primaryAction}
      onClearSelection={onClearSelection}
      showFilters={false}
      className="mb-4"
    />
  );
}
