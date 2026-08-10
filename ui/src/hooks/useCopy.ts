import { useCallback, useState } from 'react'

export function useCopy(timeout = 1500) {
  const [copied, setCopied] = useState(false)

  const copy = useCallback(async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), timeout)
    } catch {
      // clipboard unavailable — ignore
    }
  }, [timeout])

  return { copied, copy }
}