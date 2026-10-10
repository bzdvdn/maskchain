import { createContext, useContext } from 'react'
import { useAttention, type AttentionState } from '../hooks/useAttention'
import { useWorkspace } from '../hooks/useWorkspace'

const AttentionContext = createContext<AttentionState | null>(null)

// @sk-task operator-console-v2-1#T1.2: one attention source for the panel and header (DEC-001)
export function AttentionProvider({ children }: { children: React.ReactNode }) {
  const [workspace] = useWorkspace()
  const state = useAttention(workspace)
  return <AttentionContext.Provider value={state}>{children}</AttentionContext.Provider>
}

export function useAttentionState(): AttentionState {
  const state = useContext(AttentionContext)
  if (!state) {
    throw new Error('useAttentionState must be used within AttentionProvider')
  }
  return state
}
