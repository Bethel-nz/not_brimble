import { useState, useMemo, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import type { Deployment } from '../api/types'
import { useDeleteDeployment, useRedeployDeployment, useCreateDeployment } from '../api/client'
import { LogViewer } from './LogViewer'
import { ExternalLink, Trash2, ChevronDown, RotateCw, History as HistoryIcon } from 'lucide-react'

const STATUS_COLORS: Record<Deployment['status'], string> = {
  pending: 'text-[#8a8a9a]',
  building: 'text-[#7dd3fc] animate-pulse',
  built: 'text-[#7dd3fc]',
  deploying: 'text-[#7dd3fc] animate-pulse',
  running: 'text-[#4ade80]',
  failed: 'text-[#fb7185]',
  stopped: 'text-[#565665]',
}

const GLYPHS: Record<Deployment['status'], string> = {
  pending: '○',
  building: '◐',
  built: '●',
  deploying: '◉',
  running: '●',
  failed: '✗',
  stopped: '◻',
}

const timeAgo = (dateString: string) => {
  const diff = Date.now() - new Date(dateString).getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function Glyph({ status }: { status: Deployment['status'] }) {
  return <span className={`font-mono text-xs inline-block w-3 ${STATUS_COLORS[status]}`}>{GLYPHS[status]}</span>;
}

export function DeploymentRow({ deployment, history = [] }: { deployment: Deployment, history?: Deployment[] }) {
  const [isOpen, setIsOpen] = useState(false)
  const [tagsOpen, setTagsOpen] = useState(false)
  const tagsRef = useRef<HTMLDivElement>(null)
  
  const deleteMutation = useDeleteDeployment()
  const redeployMutation = useRedeployDeployment()
  const createMutation = useCreateDeployment()

  // Handle click outside for tags dropdown
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (tagsRef.current && !tagsRef.current.contains(event.target as Node)) {
        setTagsOpen(false)
      }
    }
    if (tagsOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [tagsOpen])

  // Builds excluding the current "latest" one
  const pastBuilds = useMemo(() => {
    return history.filter(h => h.id !== deployment.id)
  }, [history, deployment.id])

  // List all builds that have an image tag, ordered by date
  const tags = useMemo(() => {
    return history
      .filter(d => d.image_tag)
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  }, [history])

  const handleDelete = () => {
    if (window.confirm(`Delete service ${deployment.name || deployment.id}?`)) {
      deleteMutation.mutate(deployment.id)
    }
  }

  const handleAction = (build: Deployment) => {
    setTagsOpen(false)
    if (build.image_tag) {
      redeployMutation.mutate(build.id)
    } else if (build.source_type === 'git' && build.source_url) {
      createMutation.mutate({ sourceType: 'git', sourceUrl: build.source_url })
    }
  }

  const shortId = deployment.id.slice(0, 8).toLowerCase()
  const isPending = redeployMutation.isPending || createMutation.isPending

  return (
    <motion.div 
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      layout
      className={`transition-colors border-l-2 cursor-pointer group flex flex-col ${
        isOpen ? 'bg-[#16171d] border-[#7dd3fc]' : 'hover:bg-[#1e2028]/60 border-transparent'
      }`}
      onClick={() => setIsOpen(!isOpen)}
    >
      <div className="px-4 py-3 flex items-center gap-4">
        <div className="w-3 shrink-0">
           <Glyph status={deployment.status} />
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-mono text-[13px] text-[#f4f4f7] font-medium truncate uppercase tracking-tight">
               {deployment.name || shortId}
            </span>
            {deployment.image_tag && (
              <span className="font-mono text-[10px] text-[#565665] shrink-0 uppercase tracking-tighter bg-[#272a34]/30 px-1.5 py-0.5 rounded-sm flex items-center gap-1">
                latest
              </span>
            )}
          </div>
          <div className="font-mono text-[10px] text-[#8a8a9a] flex items-center gap-1.5 truncate mt-1">
            <span className="text-[#565665]">├</span>
            <span className="text-[#7dd3fc]/80 shrink-0 uppercase">{deployment.source_type}</span>
            <span className="text-[#565665]">·</span>
            <span className="truncate opacity-50">{deployment.source_url}</span>
          </div>
        </div>

        <div className={`font-mono text-[11px] font-bold tracking-widest uppercase w-24 shrink-0 ${STATUS_COLORS[deployment.status]}`}>
          {deployment.status}
        </div>

        <div className="font-mono text-[10px] text-[#565665] text-right tabular-nums w-20 shrink-0 uppercase tracking-tighter">
          {timeAgo(deployment.created_at)}
        </div>

        <div className="flex items-center gap-1 w-20 shrink-0 justify-end">
          {deployment.status === 'running' && deployment.subdomain ? (
            <a
              href={`http://${deployment.subdomain}`}
              target="_blank"
              rel="noreferrer"
              className="p-1.5 text-[#8a8a9a] hover:text-[#4ade80] transition-colors"
              title="Open Live URL"
              onClick={e => e.stopPropagation()}
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          ) : (
            <span className="p-1.5 text-[#23232b] cursor-not-allowed" title="Not running">
              <ExternalLink className="w-3.5 h-3.5" />
            </span>
          )}
          <button
            onClick={(e) => { e.stopPropagation(); handleDelete() }}
            disabled={deleteMutation.isPending}
            className="p-1.5 text-[#8a8a9a] hover:text-[#fb7185] transition-colors disabled:opacity-50 disabled:cursor-not-allowed opacity-0 group-hover:opacity-100"
            title="Delete Service"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
          <div className={`p-1 transition-transform duration-200 ${isOpen ? 'rotate-180 text-[#7dd3fc]' : 'text-[#565665]'}`}>
            <ChevronDown className="w-4 h-4" />
          </div>
        </div>
      </div>
      
      <AnimatePresence>
        {isOpen && (
          <motion.div 
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="px-4 pb-4 overflow-hidden"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex flex-col gap-4">
              {/* Actions Header */}
              <div className="flex items-center gap-3 border-t border-[#272a34] pt-3">
                <div className="flex gap-2">
                  <button 
                    onClick={() => handleAction(deployment)}
                    disabled={isPending}
                    className="flex items-center gap-2 px-3 py-1.5 bg-[#7dd3fc]/10 hover:bg-[#7dd3fc]/20 border border-[#7dd3fc]/30 text-[#7dd3fc] font-mono text-[10px] uppercase font-bold transition-colors disabled:opacity-50"
                  >
                    <RotateCw className={`w-3 h-3 ${isPending ? 'animate-spin' : ''}`} />
                    {deployment.image_tag ? 'Redeploy' : 'Rebuild'}
                  </button>
                  
                  {tags.length > 0 && (
                    <div className="relative" ref={tagsRef}>
                      <button 
                        onClick={() => setTagsOpen(!tagsOpen)}
                        className={`flex items-center gap-2 px-3 py-1.5 border font-mono text-[10px] uppercase font-bold transition-colors ${
                          tagsOpen ? 'bg-[#272a34] border-[#d4d6dd] text-white' : 'bg-[#272a34]/30 hover:bg-[#272a34]/50 border border-[#272a34] text-[#d4d6dd]'
                        }`}
                      >
                        Tags <ChevronDown className={`w-3 h-3 transition-transform ${tagsOpen ? 'rotate-180' : ''}`} />
                      </button>
                      
                      <AnimatePresence>
                        {tagsOpen && (
                          <motion.div 
                            initial={{ opacity: 0, y: 4 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: 4 }}
                            className="absolute left-0 top-full mt-1 w-56 bg-[#111216] border border-[#272a34] shadow-2xl z-20 py-1"
                          >
                            <div className="max-h-60 overflow-y-auto">
                              {tags.map(t => {
                                const isCurrent = t.id === deployment.id
                                return (
                                  <button
                                    key={t.id}
                                    onClick={() => handleAction(t)}
                                    className={`w-full text-left px-3 py-2 hover:bg-[#1e2028] font-mono text-[10px] transition-colors flex items-center justify-between gap-4 ${
                                      isCurrent ? 'text-[#4ade80]' : 'text-[#8a8a9a] hover:text-[#f4f4f7]'
                                    }`}
                                  >
                                    <span className="truncate">{t.image_tag.split(':')[1]?.slice(0, 7) || t.id.slice(0, 7)}</span>
                                    <div className="flex items-center gap-2 shrink-0">
                                      {isCurrent && <span className="text-[8px] font-bold uppercase bg-[#4ade80]/10 px-1 border border-[#4ade80]/20">LIVE</span>}
                                      <span className="text-[8px] opacity-40 uppercase">{timeAgo(t.created_at)}</span>
                                    </div>
                                  </button>
                                )
                              })}
                            </div>
                          </motion.div>
                        )}
                      </AnimatePresence>
                    </div>
                  )}
                </div>

                <div className="flex-1" />

                <div className="flex items-center gap-2 text-[#565665] font-mono text-[9px] uppercase font-bold tracking-widest">
                  <HistoryIcon className="w-3 h-3" />
                  Versions ({history.length})
                </div>
              </div>

              <LogViewer deploymentId={deployment.id} isOpen={isOpen} />

              {/* Version History Tray - Visible by default when expanded */}
              {pastBuilds.length > 0 && (
                <div className="border-t border-[#272a34] pt-4">
                  <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-none">
                    {pastBuilds.map(h => (
                      <div 
                        key={h.id}
                        className="w-48 shrink-0 bg-[#0a0b0d] border border-[#272a34] p-3 hover:border-[#565665] transition-colors cursor-default"
                      >
                        <div className="flex items-center justify-between mb-2">
                          <Glyph status={h.status} />
                          <span className="font-mono text-[9px] text-[#565665]">{timeAgo(h.created_at)}</span>
                        </div>
                        <div className="font-mono text-[10px] text-[#f4f4f7] font-bold mb-1 truncate">
                          {h.id.slice(0, 8).toUpperCase()}
                        </div>
                        <div className="font-mono text-[9px] text-[#565665] uppercase mb-3">
                          {h.image_tag?.split(':')[1]?.slice(0, 7) || 'no-tag'}
                        </div>
                        <button 
                          onClick={() => handleAction(h)}
                          disabled={isPending}
                          className="w-full py-1.5 border border-[#272a34] hover:border-[#7dd3fc] hover:bg-[#7dd3fc]/5 text-[#565665] hover:text-[#7dd3fc] font-mono text-[9px] uppercase font-bold transition-all disabled:opacity-30"
                        >
                          {h.image_tag ? 'Rollback' : 'Retry'}
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}
