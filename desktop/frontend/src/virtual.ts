// Virtualized list rendering: only the rows in the viewport exist in the DOM,
// so a 40,000-row file scrolls as smoothly as a 40-row one.

import { useCallback, useEffect, useRef, useState } from 'react'

export interface VirtualSlice {
  start: number
  end: number
  offsetTop: number
  totalHeight: number
  onScroll: (event: React.UIEvent<HTMLDivElement>) => void
  containerRef: React.RefObject<HTMLDivElement | null>
}

export function useVirtual(total: number, rowHeight: number, overscan = 8, active = true, resetKey?: unknown): VirtualSlice {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [windowHeight, setWindowHeight] = useState(600)
  const [scrollTop, setScrollTop] = useState(0)

  useEffect(() => {
    const container = containerRef.current

    if (!container) return

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setWindowHeight(entry.contentRect.height)
      }
    })

    observer.observe(container)

    return () => observer.disconnect()
  }, [active])

  useEffect(() => {
    if (containerRef.current) containerRef.current.scrollTop = 0
    setScrollTop(0)
  }, [resetKey, active])

  const onScroll = useCallback((event: React.UIEvent<HTMLDivElement>) => {
    setScrollTop(event.currentTarget.scrollTop)
  }, [])

  const first = Math.min(Math.max(0, total - 1), Math.max(0, Math.floor(scrollTop / rowHeight) - overscan))
  const visible = Math.ceil(windowHeight / rowHeight) + overscan * 2
  const end = Math.min(total, first + visible)

  return {
    start: first,
    end,
    offsetTop: first * rowHeight,
    totalHeight: total * rowHeight,
    onScroll,
    containerRef,
  }
}
