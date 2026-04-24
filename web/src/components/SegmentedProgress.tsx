import { useEffect, useState } from 'react'

export function SegmentedProgress({ active = true }: { active?: boolean }) {
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    if (!active) return
    const interval = setInterval(() => {
      setProgress((p) => (p + 1) % 32)
    }, 120)
    return () => clearInterval(interval)
  }, [active])

  return (
    <div className="font-mono flex items-center gap-1.5 text-xs">
      <span className="text-[#4a4d58]">[</span>
      <div className="flex gap-[1px] flex-1">
        {Array.from({ length: 32 }).map((_, i) => {
          const isFilled = active && i < progress
          const isHead = active && i === progress - 1
          
          return (
             <div
               key={i}
               className={`w-1.5 h-3 ${
                 isHead ? 'opacity-100' : 
                 isFilled ? 'opacity-100' : 'opacity-20'
               }`}
               style={{
                 background: isFilled ? '#4ade80' : '#272a34'
               }}
             />
          )
        })}
      </div>
      <span className="text-[#4a4d58]">]</span>
    </div>
  )
}
