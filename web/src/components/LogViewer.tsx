import { useEffect, useRef } from 'react'
import { motion } from 'motion/react'
import { useLogsSSE } from '../api/sse'

interface LogViewerProps {
  deploymentId: string
  isOpen: boolean
}

export function LogViewer({ deploymentId, isOpen }: LogViewerProps) {
  const { logs, isStreaming } = useLogsSSE(deploymentId, isOpen)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 50

    if (isAtBottom) {
      container.scrollTop = container.scrollHeight
    }
  }, [logs])

  if (!isOpen) return null

  return (
    <motion.div
      initial={{ height: 0, opacity: 0 }}
      animate={{ height: 'auto', opacity: 1 }}
      exit={{ height: 0, opacity: 0 }}
      className="bg-[#0a0b0d] rounded-sm overflow-hidden border border-[#272a34] shadow-inner"
    >
      <div className="flex items-center justify-between px-4 py-2 bg-[#111216] border-b border-[#272a34]">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-[#565665] uppercase tracking-[0.2em] font-bold">Build Stream</span>
          {isStreaming && (
            <span className="inline-flex items-center gap-1.5 text-[9px] font-bold text-[#7dd3fc] bg-[#7dd3fc]/10 px-2 py-0.5 rounded-sm border border-[#7dd3fc]/20 uppercase">
               <span className="w-1 h-1 rounded-full bg-[#7dd3fc] animate-pulse" />
               following
            </span>
          )}
        </div>
      </div>
      
      <div 
        ref={containerRef}
        className="p-5 max-h-[320px] overflow-y-auto font-mono text-[11px] leading-[1.7] bg-[#07080a]"
      >
        {logs.length === 0 && !isStreaming ? (
          <div className="text-[#565665] italic text-center py-4 uppercase tracking-widest text-[9px]">— empty log session —</div>
        ) : (
          logs.map((log, i) => {
            const isErr = log.stream === 'stderr' || /fail|error|exit 1/i.test(log.line)
            const isOk = /✓|ready|success/i.test(log.line)
            const isAcc = /^→|\[build\]/.test(log.line)
            const colorClass = isErr ? 'text-[#fb7185]' : isOk ? 'text-[#4ade80]' : isAcc ? 'text-[#7dd3fc]' : 'text-[#d8d8e0]/70'
            
            return (
              <div 
                key={`${log.id}-${i}`} 
                className={`flex gap-4 ${colorClass}`}
              >
                <span className="text-[#565665] tabular-nums select-none shrink-0 w-8 text-right font-normal opacity-50">
                  {i + 1}
                </span>
                <span className="whitespace-pre-wrap break-all tracking-tight">
                  {log.line}
                </span>
              </div>
            )
          })
        )}
        {isStreaming && <span className="caret inline-block w-[6px] h-[10px] bg-[#7dd3fc] animate-pulse ml-[48px] align-baseline"/>}
      </div>
    </motion.div>
  )
}
