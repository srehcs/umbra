import * as React from 'react';
import { cn } from '@/lib/utils';

type EmptyStateProps = {
  message: string;
  className?: string;
};

export default function EmptyState({ message, className }: EmptyStateProps) {
  return (
    <div className={cn('text-sm text-muted-foreground', className)}>
      {message}
    </div>
  );
}
