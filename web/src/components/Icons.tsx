export function ISch({ size = 12, cls = '' }: { size?: number; cls?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" className={cls} aria-hidden>
      <circle cx="6.5" cy="6.5" r="4.5" stroke="currentColor" strokeWidth="1.5"/>
      <path d="M10 10l3.5 3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  )
}

export function IUp({ size = 12, cls = '' }: { size?: number; cls?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" className={cls} aria-hidden>
      <path d="M8 11V5M5 8l3-3 3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M3 13h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  )
}

export function IGit({ size = 12, cls = '' }: { size?: number; cls?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" className={cls} aria-hidden>
      <path d="M15.1 7.38L8.62.9a1.26 1.26 0 00-1.77 0L5.3 2.44l2.24 2.24a1.5 1.5 0 011.9 1.9l2.16 2.16a1.5 1.5 0 11-.9.9L8.65 7.7v4.24a1.5 1.5 0 11-1.23-.04V7.6A1.5 1.5 0 017 6.1L4.8 3.9.9 7.8a1.26 1.26 0 000 1.77l6.48 6.48a1.26 1.26 0 001.77 0l6-6a1.26 1.26 0 000-1.67z"/>
    </svg>
  )
}
