import { useState, useEffect, useRef, useMemo } from 'react'
import { useDeployments, useDeleteDeployment, useCreateDeployment, useRedeployDeployment } from '../api/client'
import { useStatusSSE, useLogsSSE } from '../api/sse'
import { ISch, IUp } from './Icons'
import type { Deployment } from '../api/types'
import { ExternalLink } from 'lucide-react'

// Glyph and Status Maps
const GLYPH_MAP: Record<string, any> = {
  running:   { ch:'●', cls:'text-t-ok' },
  building:  { ch:'◐', cls:'text-t-acc animate-spin', spin:true },
  deploying: { ch:'◉', cls:'text-t-acc dot-pulse' },
  pending:   { ch:'○', cls:'text-t-mid dot-pulse' },
  failed:    { ch:'✗', cls:'text-t-err' },
  stopped:   { ch:'◻', cls:'text-t-dim' },
}
const STATUS_LABEL: Record<string, string> = { running:'READY', building:'BUILD', deploying:'DPLY', pending:'WAIT', failed:'FAIL', stopped:'STOP' }
const STATUS_COLOR: Record<string, string> = { running:'text-t-ok', building:'text-t-acc', deploying:'text-t-acc', pending:'text-t-mid', failed:'text-t-err', stopped:'text-t-dim' }

function Glyph({ status }: { status: string }) {
  const g = GLYPH_MAP[status] || GLYPH_MAP.stopped;
  return <span className={`mono text-xs inline-block w-3 ${g.cls}`} style={g.spin?{animationDuration:'2s'}:{}}>{g.ch}</span>;
}

function timeAgo(dateString: string) {
  const diff = Date.now() - new Date(dateString).getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

function formatDuration(start: string, end: string) {
  const s = new Date(start).getTime()
  const e = new Date(end).getTime()
  const diff = Math.floor((e - s) / 1000)
  if (diff < 0) return '---'
  if (diff < 60) return `${diff}s`
  return `${Math.floor(diff / 60)}m ${diff % 60}s`
}

export function TerminalApp() {
  const { data: deployments = [], isLoading } = useDeployments()
  useStatusSSE(deployments)

  const [selRepoKey, setSelRepoKey] = useState<string | null>(null)
  const [selBuildId, setSelBuildId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const [showKbar, setShowKbar] = useState(false)
  const [showNew, setShowNew] = useState(false)
  const [toast, setToast] = useState<string | null>(null)
  
  const listRef = useRef<HTMLDivElement>(null)

  const repos = useMemo(() => {
    const groups: Record<string, Deployment[]> = {}
    deployments.forEach(d => {
      const key = d.source_url || 'upload-' + d.id
      if (!groups[key]) groups[key] = []
      groups[key].push(d)
    })
    
    Object.values(groups).forEach(builds => {
      builds.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    })

    return Object.entries(groups).map(([url, builds]) => ({
      url,
      name: builds[0].name || 'unknown',
      latestStatus: builds[0].status,
      lastBuildAt: builds[0].created_at,
      builds
    })).sort((a, b) => new Date(b.lastBuildAt).getTime() - new Date(a.lastBuildAt).getTime())
  }, [deployments])

  const filteredRepos = useMemo(() => {
    let list = repos
    if (filter !== 'all') list = list.filter(r => r.latestStatus === filter)
    if (query) {
      const q = query.toLowerCase()
      list = list.filter(r => 
        (r.name + r.url).toLowerCase().includes(q)
      )
    }
    return list
  }, [repos, filter, query])

  useEffect(() => {
    if (!selRepoKey && filteredRepos.length > 0) {
      setSelRepoKey(filteredRepos[0].url)
      setSelBuildId(filteredRepos[0].builds[0].id)
    }
  }, [filteredRepos, selRepoKey])

  const selectedRepo = repos.find(r => r.url === selRepoKey) || null
  const selectedBuild = selectedRepo?.builds.find(b => b.id === selBuildId) || selectedRepo?.builds[0] || null

  useEffect(() => {
    if (toast) { const t = setTimeout(() => setToast(null), 2500); return () => clearTimeout(t); }
  }, [toast]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (['INPUT','TEXTAREA'].includes(document.activeElement?.tagName || '')) {
        if (e.key === 'Escape') (document.activeElement as HTMLElement).blur();
        return;
      }
      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        const idx = filteredRepos.findIndex(r => r.url === selRepoKey);
        const next = filteredRepos[Math.min(idx+1, filteredRepos.length-1)];
        if (next) {
          setSelRepoKey(next.url)
          setSelBuildId(next.builds[0].id)
        }
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        const idx = filteredRepos.findIndex(r => r.url === selRepoKey);
        const next = filteredRepos[Math.max(idx-1, 0)];
        if (next) {
          setSelRepoKey(next.url)
          setSelBuildId(next.builds[0].id)
        }
      } else if (e.key === 'n') {
        e.preventDefault();
        setShowNew(true);
      } else if (e.key === '/') {
        e.preventDefault();
        document.getElementById('search')?.focus();
      } else if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setShowKbar(true);
      } else if (e.key === 'c' && selectedBuild && selectedBuild.status === 'running' && selectedBuild.subdomain) {
        navigator.clipboard?.writeText('http://'+selectedBuild.subdomain);
        setToast('URL copied');
      } else if (e.key === 'Escape') {
        setShowKbar(false); setShowNew(false);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [filteredRepos, selRepoKey, selectedBuild]);

  return (
    <div className="h-screen bg-t-bg flex flex-col text-t-fg overflow-hidden selection:bg-t-acc/30 font-sans">
      <header className="h-10 bg-t-panel border-b border-t-rule flex items-center px-3 gap-3 shrink-0">
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 bg-white flex items-center justify-center shrink-0">
            <div className="w-2 h-2 bg-zinc-950"></div>
          </div>
          <span className="font-semibold tracking-tight text-sm text-t-hi">not brimble</span>
        </div>
        <div className="w-px h-4 bg-t-rule"/>
        <div className="mono text-[11px] text-t-mid flex items-center gap-2 uppercase tracking-widest font-bold">
          <span className="text-t-dim">WORKSPACE</span>
          <span className="text-t-fg font-bold underline decoration-t-acc/30">PERSONAL</span>
          <span className="text-t-dim">/</span>
          <span className="text-t-fg tracking-tight text-[9px]">Production</span>
        </div>
        <div className="flex-1"/>
        <button onClick={()=>setShowKbar(true)} className="flex items-center gap-1.5 mono text-[11px] text-t-mid hover:text-t-fg transition px-2 py-1 border border-t-rule cursor-pointer active:bg-t-line">
          <ISch size={11}/>
          <span>COMMANDS</span>
          <kbd className="text-[10px] text-t-dim bg-t-bg border border-t-rule px-1 py-0.5 ml-1">⌘K</kbd>
        </button>
        <div className="mono text-[10px] text-t-dim flex items-center gap-1.5 ml-2">
          <span className="w-1.5 h-1.5 bg-t-ok dot-pulse"/>
          CONNECTED
        </div>
      </header>

      <div className="h-9 bg-t-panel/60 border-b border-t-rule flex items-center px-3 gap-3 shrink-0">
        <div className="flex items-center gap-1 mono text-[11px]">
          {[['all'],['running'],['building'],['failed']].map(([f]) => (
            <button key={f} onClick={()=>setFilter(f)}
                    className={`px-2 py-0.5 transition cursor-pointer ${filter===f?'bg-t-line text-t-hi':'text-t-mid hover:text-t-fg'}`}>
              {f.toUpperCase()}
              <span className="text-t-dim ml-1">({
                f==='all'?repos.length:
                repos.filter(r=>r.latestStatus===f).length
              })</span>
            </button>
          ))}
        </div>
        <div className="flex-1 flex items-center gap-2 bg-t-bg border border-t-rule px-2 h-6 max-w-md focus-within:border-t-acc/50 transition-colors text-[#4ade80]">
          <ISch size={11} cls="text-t-dim"/>
          <input id="search" type="text" value={query} onChange={e=>setQuery(e.target.value)}
                 placeholder="filter instances…"
                 className="flex-1 bg-transparent mono text-[11px] text-t-fg placeholder:text-t-dim outline-none"/>
          <kbd className="text-[10px] text-t-dim opacity-50">/</kbd>
        </div>
        <button onClick={()=>setShowNew(true)} className="mono text-[11px] text-[#0a0b0d] bg-t-acc hover:bg-white border border-t-acc px-3 h-6 inline-flex items-center gap-1.5 cursor-pointer transition-colors font-bold uppercase shadow-[0_0_15px_rgba(74,222,128,0.2)]">
          <span>+</span>
          init service
          <kbd className="text-[10px] text-[#0a0b0d]/50 ml-1">n</kbd>
        </button>
      </div>

      <div className="flex-1 grid min-h-0" style={{gridTemplateColumns:'minmax(420px, 35%) 1fr'}}>
        <div className="border-r border-t-rule overflow-y-auto flex flex-col bg-t-panel/20" ref={listRef}>
          <div className="mono text-[10px] text-t-dim px-3 py-2 border-b border-t-rule bg-t-panel/40 grid items-center gap-3 shrink-0 uppercase tracking-widest font-bold" style={{gridTemplateColumns:'16px 1fr 50px 30px'}}>
            <div/>
            <div>SERVICE</div>
            <div>STATUS</div>
            <div className="text-right">AGE</div>
          </div>
          
          <div className="flex-1 overflow-y-auto">
            {isLoading && (
              <div className="mono text-xs text-t-dim text-center py-12 flex flex-col items-center gap-4 animate-pulse uppercase">
                <span>INITIALIZING SESSION...</span>
              </div>
            )}
            
            {filteredRepos.map(repo => {
              const isSelected = selRepoKey === repo.url
              return (
                <div key={repo.url}
                     onClick={()=>{setSelRepoKey(repo.url); setSelBuildId(repo.builds[0].id)}}
                     className={`tr-row cursor-pointer px-3 py-3 hover:bg-t-line/40 transition-colors ${isSelected?'sel':''}`}>
                  <div className="grid items-center gap-3" style={{gridTemplateColumns:'16px 1fr 50px 30px'}}>
                    <Glyph status={repo.latestStatus}/>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="mono text-[13px] text-t-hi font-bold truncate tracking-tight uppercase">{repo.name}</span>
                        <span className="mono text-[9px] text-t-dim border border-t-rule px-1 uppercase tracking-tighter opacity-70">{repo.builds.length}V</span>
                      </div>
                      <div className="mono text-[10px] text-t-mid flex items-center gap-1.5 truncate mt-0.5 opacity-60 font-medium uppercase">
                        <span className="truncate">{repo.url.replace('https://github.com/','')}</span>
                      </div>
                    </div>
                    <div className={`mono text-[10px] font-bold tracking-widest ${STATUS_COLOR[repo.latestStatus]}`}>
                      {STATUS_LABEL[repo.latestStatus]}
                    </div>
                    <div className="mono text-[10px] text-t-dim text-right tabular-nums">{timeAgo(repo.lastBuildAt)}</div>
                  </div>
                </div>
              )
            })}
            
            {!isLoading && filteredRepos.length === 0 && (
              <div className="mono text-xs text-t-dim text-center py-12 opacity-50 uppercase tracking-[0.2em]">— empty —</div>
            )}
          </div>

          <div className="shrink-0 bg-t-panel border-t border-t-rule mono text-[10px] text-t-dim px-3 py-2 flex items-center gap-3 font-bold uppercase tracking-widest text-center justify-center">
             <span>CTRL :: NAV [ J / K ] · INIT [ N ] · SEARCH [ / ]</span>
          </div>
        </div>

        <div className="flex flex-col min-h-0 overflow-hidden bg-t-bg">
          {selectedBuild ? (
            <Detail
              d={selectedBuild}
              allBuilds={selectedRepo?.builds || []}
              onSelectBuild={setSelBuildId}
              onCopy={(u)=>{navigator.clipboard?.writeText('http://'+u);setToast('URL copied');}}
              onToast={setToast}
              onNewBuild={(id) => setSelBuildId(id)}
            />
          ) : (
            <div className="flex-1 flex items-center justify-center mono text-sm text-t-dim uppercase tracking-[0.5em] animate-pulse">
               — session idle —
            </div>
          )}
        </div>
      </div>

      {showNew && <NewDeploy onClose={()=>setShowNew(false)} onToast={setToast} />}
      {showKbar && <Kbar onClose={()=>setShowKbar(false)} onAction={(a)=>{
        if (a==='new') setShowNew(true)
        if (a==='search') document.getElementById('search')?.focus()
        setShowKbar(false)
      }}/>}

      {toast && (
        <div className="fixed bottom-8 left-1/2 -translate-x-1/2 z-50 kbar-in">
          <div className="mono bg-[#111114] border border-t-acc/60 text-t-acc text-[11px] font-bold px-8 py-3 shadow-[0_0_50px_rgba(0,0,0,0.9)] tracking-[0.2em]">
             <span className="mr-3 opacity-50">›</span> {toast.toUpperCase()}
          </div>
        </div>
      )}
    </div>
  )
}

function Detail({
  d,
  allBuilds,
  onSelectBuild,
  onCopy,
  onToast,
  onNewBuild,
}: {
  d: Deployment
  allBuilds: Deployment[]
  onSelectBuild: (id: string) => void
  onCopy: (url: string) => void
  onToast: (t: string) => void
  onNewBuild: (id: string) => void
}) {
  const stopMutation = useDeleteDeployment()
  const redeployMutation = useRedeployDeployment()
  const createMutation = useCreateDeployment()
  const { logs, isStreaming } = useLogsSSE(d.id, true)
  const logsRef = useRef<HTMLDivElement>(null)
  const [versionActionId, setVersionActionId] = useState<string | null>(null)

  useEffect(() => {
    if (logsRef.current) logsRef.current.scrollTop = logsRef.current.scrollHeight
  }, [logs])

  const handleStop = () => {
    stopMutation.mutate(d.id, { onSuccess: () => onToast('Shutdown queued') })
  }

  const handleRollback = (build: Deployment) => {
    if (build.image_tag) {
      redeployMutation.mutate(build.id, {
        onSuccess: (newDep) => { onToast('Rollback queued'); onNewBuild(newDep.id) },
      })
    } else {
      if (d.source_url) {
        createMutation.mutate({ sourceType: 'git', sourceUrl: d.source_url }, {
          onSuccess: (newDep) => { onToast('Rebuild queued'); onNewBuild(newDep.id) },
        })
      } else {
        onToast('No source available to rebuild')
      }
    }
  }

  const duration = (['running', 'failed', 'stopped'].includes(d.status))
    ? formatDuration(d.created_at.toString(), d.updated_at.toString())
    : 'in progress'

  const showVersions = allBuilds.length > 1
  const isInProgress = ['pending', 'building', 'built', 'deploying'].includes(d.status)

  return (
    <div className="flex flex-col min-h-0 h-full bg-[#0a0b0d]">
      {/* Header */}
      <div className="px-5 py-4 border-b border-t-rule bg-t-panel/30 shrink-0">
        <div className="flex items-center gap-3">
          <Glyph status={d.status}/>
          <span className="mono text-sm text-t-hi font-bold tracking-tight uppercase">{d.name || d.id.slice(0,8)}</span>
          <span className="mono text-[10px] text-t-dim opacity-60 uppercase">ID:{d.id}</span>
          <div className="flex-1"/>

          {d.status === 'running' && d.subdomain && (
            <button onClick={()=>onCopy(d.subdomain)} className="mono text-[11px] text-t-mid hover:text-t-fg transition inline-flex items-center gap-1 px-3 py-1 border border-t-rule cursor-pointer active:bg-t-line uppercase tracking-wider">
              <ExternalLink size={11} className="mr-0.5"/><span>url</span><kbd className="text-[9px] text-t-dim ml-1">c</kbd>
            </button>
          )}

          {!isInProgress && (
            <button
              onClick={handleStop}
              disabled={stopMutation.isPending}
              className="mono text-[11px] text-t-mid hover:text-t-err transition inline-flex items-center gap-1 px-3 py-1 border border-t-rule cursor-pointer disabled:opacity-50 uppercase tracking-wider"
            >
              {d.status === 'running' ? 'stop' : 'remove'}
              <kbd className="text-[9px] text-t-dim ml-1">d</kbd>
            </button>
          )}

          {isInProgress && (
            <button
              onClick={handleStop}
              disabled={stopMutation.isPending}
              className="mono text-[11px] text-t-dim hover:text-t-err transition inline-flex items-center gap-1 px-3 py-1 border border-t-rule cursor-pointer disabled:opacity-50 uppercase tracking-wider"
            >
              cancel
            </button>
          )}
        </div>

        <div className="mt-4 grid grid-cols-4 gap-6 border border-t-rule bg-t-bg/50 p-4">
          <Meta label="source" value={d.source_type} acc uppercase />
          <Meta label="public url" value={d.status === 'running' ? "http://" + d.subdomain : '---'} mono acc={d.status==='running'} />
          <Meta label="image tag" value={d.image_tag || '---'} />
          <Meta label="duration" value={duration} acc />
        </div>

        <div className="mt-4 mono text-[11px] text-t-mid flex items-center gap-2">
          <span className="text-t-dim font-bold uppercase tracking-widest text-[9px] shrink-0">Source Path</span>
          <span className="text-t-fg/60 truncate italic px-2 bg-t-panel/40">"{d.source_url}"</span>
        </div>
      </div>

      {/* Logs */}
      <div className="flex-1 overflow-hidden flex flex-col min-h-0">
        <div className="flex items-center gap-2 px-5 py-2.5 border-b border-t-rule bg-t-panel/10 mono text-[9px] text-t-dim uppercase tracking-[0.2em] shrink-0 font-bold">
          <span>Stream :: Build & Runtime</span>
          {isStreaming && (
            <span className="inline-flex items-center gap-1.5 text-t-acc ml-2">
              <span className="w-1 h-1 bg-t-acc dot-pulse"/>
              following
            </span>
          )}
        </div>
        <div ref={logsRef} className="flex-1 overflow-y-auto mono text-[12px] leading-[1.65] px-5 py-4 min-h-0 bg-[#07080a]">
          {logs.length === 0 && !isStreaming && (
            <div className="text-t-dim italic text-center py-10 opacity-30 tracking-widest uppercase text-[10px]">— journal empty —</div>
          )}
          {logs.map((log, i) => {
            const isErr = log.stream === 'stderr' || /fail|error|exit 1/i.test(log.line)
            const isOk = /✓|ready|success/i.test(log.line)
            const isAcc = /^→|\[build\]/.test(log.line)
            const colorClass = isErr ? 'text-t-err' : isOk ? 'text-t-ok' : isAcc ? 'text-t-acc' : 'text-t-fg/80'
            return (
              <div key={`${log.id}-${i}`} className={`log-line flex gap-4 ${colorClass}`}>
                <span className="text-t-dim tabular-nums select-none shrink-0 w-10 text-right opacity-40 font-normal">{i + 1}</span>
                <span className="whitespace-pre-wrap break-all tracking-tight">{log.line}</span>
              </div>
            )
          })}
          {isStreaming && <span className="caret mono text-t-acc ml-[56px] opacity-80">▋</span>}
        </div>
      </div>

      {/* Version history tray */}
      {showVersions && (
        <div className="shrink-0 border-t border-t-rule bg-t-panel/20 flex flex-col shadow-[0_-8px_30px_rgba(0,0,0,0.4)]">
          <div className="px-4 py-1.5 border-b border-t-rule bg-t-panel/40 mono text-[9px] text-t-dim uppercase tracking-[0.25em] font-bold flex items-center gap-2">
            <span>Versions</span>
            <span className="text-t-fg/30">·</span>
            <span>{allBuilds.length} builds</span>
            <span className="text-t-fg/30 ml-auto text-[8px]">click to expand actions</span>
          </div>
          <div className="overflow-x-auto flex items-stretch p-3 gap-2 scrollbar-none" style={{height: versionActionId ? 'auto' : '7.5rem', maxHeight: '13rem'}}>
            {allBuilds.map((b, i) => {
              const isCurrent = i === 0
              const isViewing = d.id === b.id
              const isExpanded = versionActionId === b.id
              const canRollback = !isCurrent && (b.status === 'running' || b.status === 'stopped') && !!b.image_tag
              const canRedeploy = !isCurrent && b.status === 'failed'

              return (
                <div
                  key={b.id}
                  className={`w-40 shrink-0 border flex flex-col transition-all cursor-pointer ${
                    isExpanded
                      ? 'bg-t-line border-t-hi'
                      : isViewing
                      ? 'bg-t-line/60 border-t-acc/50'
                      : 'bg-t-bg border-t-rule hover:border-t-dim'
                  }`}
                  onClick={() => {
                    onSelectBuild(b.id)
                    if (!isCurrent) setVersionActionId(isExpanded ? null : b.id)
                  }}
                >
                  <div className="p-2 flex-1">
                    <div className="flex items-center justify-between mb-1">
                      <Glyph status={b.status} />
                      {isCurrent && (
                        <span className="mono text-[8px] text-t-acc font-bold tracking-widest border border-t-acc/30 px-1">LIVE</span>
                      )}
                      <span className="mono text-[9px] text-t-dim">{timeAgo(b.created_at)}</span>
                    </div>
                    <div className="mono text-[10px] text-t-hi truncate font-bold uppercase">{b.id.slice(0, 10)}</div>
                    <div className="mono text-[9px] text-t-dim truncate mt-0.5 opacity-50 uppercase font-bold">
                      {['running','failed','stopped'].includes(b.status) ? formatDuration(b.created_at, b.updated_at) : '···'}
                    </div>
                  </div>

                  {/* Inline action — only when expanded and not current */}
                  {isExpanded && !isCurrent && (
                    <div className="border-t border-t-rule p-1.5 space-y-1" onClick={e => e.stopPropagation()}>
                      {canRollback && (
                        <button
                          onClick={() => handleRollback(b)}
                          disabled={redeployMutation.isPending}
                          className="w-full mono text-[9px] text-t-hi border border-t-rule py-1.5 hover:bg-t-hi/10 hover:border-t-hi transition font-bold uppercase tracking-wider disabled:opacity-40 cursor-pointer"
                        >
                          ↩ rollback to this
                        </button>
                      )}
                      {canRedeploy && (
                        <button
                          onClick={() => handleRollback(b)}
                          disabled={createMutation.isPending}
                          className="w-full mono text-[9px] text-t-hi border border-t-rule py-1.5 hover:bg-t-hi/10 hover:border-t-hi transition font-bold uppercase tracking-wider disabled:opacity-40 cursor-pointer"
                        >
                          ⟳ redeploy build
                        </button>
                      )}
                      {!canRollback && !canRedeploy && (
                        <div className="w-full mono text-[9px] text-t-dim text-center py-1.5 uppercase tracking-wider opacity-50">
                          {isCurrent ? 'current version' : 'no action available'}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

function Meta({ label, value, acc, mono=true, uppercase=false }: any) {
  return (
    <div className="overflow-hidden">
      <div className="text-t-dim text-[9px] uppercase tracking-[0.15em] font-bold mb-1">{label}</div>
      <div className={`${mono?'mono':''} truncate ${acc?'text-t-acc':'text-t-fg'} ${uppercase?'uppercase':''} text-[12px] font-medium tracking-tight`}>
        {value}
      </div>
    </div>
  );
}

function NewDeploy({ onClose, onToast }: { onClose: () => void, onToast: (t: string) => void }) {
  const [tab, setTab] = useState<'git'|'upload'>('git')
  const [url, setUrl] = useState('')
  const [file, setFile] = useState<File|null>(null)
  
  const createMutation = useCreateDeployment()
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { inputRef.current?.focus() }, [])

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if ((tab==='git'&&!url)||(tab==='upload'&&!file)) return;
    
    if (tab === 'git') {
      createMutation.mutate({ sourceType: 'git', sourceUrl: url }, {
        onSuccess: () => { onToast('Provisioning instance...'); onClose() }
      })
    } else if (tab === 'upload' && file) {
      createMutation.mutate({ sourceType: 'upload', file }, {
        onSuccess: () => { onToast('Provisioning instance...'); onClose() }
      })
    }
  }

  return (
    <div className="fixed inset-0 bg-t-bg/85 backdrop-blur-md z-40 flex items-start justify-center pt-[18vh] p-4" onClick={onClose}>
      <div onClick={e=>e.stopPropagation()} className="kbar-in w-full max-w-lg bg-t-panel border border-t-rule shadow-[0_30px_100px_rgba(0,0,0,0.6)]">
        <div className="px-5 py-4 border-b border-t-rule flex items-center gap-2 bg-t-panel/50">
          <span className="mono text-t-acc font-bold">▲</span>
          <span className="mono text-[12px] text-t-hi font-bold uppercase tracking-[0.1em]">Provision New Instance</span>
          <div className="flex-1"/>
          <div className="flex gap-0.5 p-0.5 bg-t-bg border border-t-rule">
            {['git','upload'].map(t=>(
              <button key={t} type="button" onClick={()=>setTab(t as any)}
                      className={`mono text-[10px] px-3 py-1 transition inline-flex items-center gap-1 cursor-pointer uppercase font-bold ${tab===t?'bg-t-line text-t-hi':'text-t-mid hover:text-t-fg'}`}>
                {t}
              </button>
            ))}
          </div>
          <kbd className="mono text-[9px] text-t-dim bg-t-bg border border-t-rule px-1.5 py-0.5 ml-2">ESC</kbd>
        </div>
        <form onSubmit={submit} className="p-6 space-y-5">
          {tab==='git' ? (
            <div>
              <div className="mono text-[10px] text-t-dim uppercase tracking-[0.15em] mb-2 font-bold">Repository Link</div>
              <div className="flex items-center gap-2 bg-t-bg border border-t-rule px-3 py-2 focus-within:border-t-acc/50 transition-colors shadow-inner text-[#4ade80]">
                <span className="mono text-t-acc font-bold opacity-60">›</span>
                <input ref={inputRef} type="text" value={url} onChange={e=>setUrl(e.target.value)}
                       placeholder="https://github.com/acme/service"
                       className="flex-1 bg-transparent mono text-[13px] text-t-fg placeholder:text-t-dim outline-none"/>
              </div>
            </div>
          ) : (
            <div>
              <div className="mono text-[10px] text-t-dim uppercase tracking-[0.15em] mb-2 font-bold">Archive Package</div>
              <label className="flex items-center gap-3 bg-t-bg border border-dashed border-t-rule px-4 py-3 cursor-pointer hover:border-t-acc/50 transition-colors shadow-inner group">
                <IUp size={14} cls="text-t-dim group-hover:text-t-acc" />
                <span className={`mono text-[12px] truncate ${file?'text-t-fg font-bold':'text-t-dim italic'}`}>{file?.name||'Select .tar.gz or .zip archive...'}</span>
                <input type="file" accept=".tar.gz,.tgz,.zip,application/gzip,application/x-gzip,application/x-tar,application/zip" className="hidden" onChange={e => setFile(e.target.files?.[0]||null)}/>
              </label>
            </div>
          )}
          
          {createMutation.isError && (
             <div className="mono text-[10px] text-t-err pt-1 font-bold uppercase tracking-wider">! Provisioning fault detected. Check endpoint health.</div>
          )}

          <div className="flex items-center gap-4 pt-4">
            <div className="flex-1 mono text-[9px] text-t-dim leading-relaxed tracking-tight">
              <span className="text-t-acc font-bold uppercase mr-2">NOTICE:</span> Instance will be auto-assigned a unique ULID and routed via Caddy dynamic upstream.
            </div>
            <button type="submit" disabled={createMutation.isPending||(tab==='git'&&!url)||(tab==='upload'&&!file)}
                    className="mono text-[11px] text-[#0a0b0d] bg-t-acc hover:bg-white border border-t-acc disabled:opacity-30 disabled:cursor-not-allowed px-5 py-2 transition-all inline-flex items-center gap-2 font-bold uppercase tracking-widest shadow-[0_0_20px_rgba(125,211,252,0.2)]">
              {createMutation.isPending?<><span className="w-2.5 h-2.5 border-2 border-[#0a0b0d]/30 border-t-[#0a0b0d] rounded-full animate-spin"/>Working</>:<>Initialize</>}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function Kbar({ onClose, onAction }: { onClose: () => void, onAction: (id: string) => void }) {
  const [q, setQ] = useState('')
  const actions = [
    { id:'new', label:'Initialize service', hint:'n', icon:'▲' },
    { id:'search', label:'Focus search', hint:'/', icon:'⌕' },
  ].filter(a => !q || a.label.toLowerCase().includes(q.toLowerCase()))
  
  const inputRef = useRef<HTMLInputElement>(null)
  useEffect(() => { inputRef.current?.focus() }, [])

  return (
    <div className="fixed inset-0 bg-t-bg/80 backdrop-blur-sm z-40 flex items-start justify-center pt-[18vh] p-4" onClick={onClose}>
      <div onClick={e=>e.stopPropagation()} className="kbar-in w-full max-w-md bg-[#111114] border border-t-rule shadow-[0_40px_120px_rgba(0,0,0,0.8)] overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-3.5 border-b border-t-rule bg-t-panel/50">
          <ISch size={14} cls="text-t-dim"/>
          <input ref={inputRef} value={q} onChange={e=>setQ(e.target.value)}
                 placeholder="Search commands..."
                 className="flex-1 bg-transparent mono text-[14px] text-t-hi placeholder:text-t-dim outline-none tracking-tight"/>
          <kbd className="mono text-[9px] text-t-dim bg-t-bg border border-t-rule px-1.5 py-0.5">ESC</kbd>
        </div>
        <div className="py-1 max-h-80 overflow-y-auto bg-t-bg/20">
          {actions.map((a, i) => (
            <button key={a.id} onClick={()=>onAction(a.id)}
                    className={`w-full flex items-center gap-4 px-4 py-3 mono text-[12px] text-t-fg hover:bg-t-line transition cursor-pointer group ${i===0?'bg-t-line/30':''}`}>
              <span className="text-t-acc w-4 text-center font-bold opacity-60 group-hover:opacity-100">{a.icon}</span>
              <span className="flex-1 text-left uppercase tracking-wider font-medium group-hover:text-white">{a.label}</span>
              {a.hint && <kbd className="text-[9px] text-t-dim bg-t-bg border border-t-rule rounded-sm px-1.5 py-0.5 group-hover:border-t-dim group-hover:text-t-mid">{a.hint}</kbd>}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
