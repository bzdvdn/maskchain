import type { ReactNode } from 'react'
import { Button } from './Button'
import { EmptyState } from './EmptyState'
import { TableSkeleton } from './TableSkeleton'

// @sk-task ui-production-readiness#T2.2: Unified loading/empty/error contract (AC-006)
//
// AsyncSection renders the console's single async-state contract: a skeleton
// while loading, a meaningful empty state, and an error state with a retry.
// Use `as="tbody"` inside tables so the states render as a full-width row.
export interface AsyncSectionProps {
  loading: boolean
  error?: unknown
  onRetry?: () => void
  empty?: boolean
  emptyMessage?: string
  emptyTitle?: string
  emptyAction?: ReactNode
  skeleton?: ReactNode
  colSpan?: number
  as?: 'div' | 'tbody'
  children: ReactNode
}

export function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  return 'Something went wrong while loading this data.'
}

function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  return (
    <EmptyState
      title="Couldn't load this data"
      message={errorMessage(error)}
      action={onRetry ? <Button onClick={onRetry}>Retry</Button> : undefined}
    />
  )
}

export function AsyncSection({
  loading,
  error,
  onRetry,
  empty = false,
  emptyMessage = 'Nothing here yet.',
  emptyTitle,
  emptyAction,
  skeleton,
  colSpan = 1,
  as = 'div',
  children,
}: AsyncSectionProps) {
  if (as === 'tbody') {
    if (loading) {
      return (
        <tbody>
          <tr>
            <td colSpan={colSpan}>{skeleton ?? <TableSkeleton />}</td>
          </tr>
        </tbody>
      )
    }
    if (error) {
      return (
        <tbody>
          <tr>
            <td colSpan={colSpan}>
              <ErrorState error={error} onRetry={onRetry} />
            </td>
          </tr>
        </tbody>
      )
    }
    if (empty) {
      return (
        <tbody>
          <tr>
            <td colSpan={colSpan}>
              <EmptyState title={emptyTitle} message={emptyMessage} action={emptyAction} />
            </td>
          </tr>
        </tbody>
      )
    }
    return <tbody>{children}</tbody>
  }

  if (loading) return <>{skeleton ?? <TableSkeleton />}</>
  if (error) return <ErrorState error={error} onRetry={onRetry} />
  if (empty) return <EmptyState title={emptyTitle} message={emptyMessage} action={emptyAction} />
  return <>{children}</>
}
