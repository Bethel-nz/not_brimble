import { useMemo } from 'react'
import { useDeployments } from '../api/client'
import { useStatusSSE } from '../api/sse'
import { DeploymentRow } from './DeploymentRow'
import { Server } from 'lucide-react'
import { SegmentedProgress } from './SegmentedProgress'
import type { Deployment } from '../api/types'

export function DeploymentList() {
  const { data: deployments, isLoading, isError } = useDeployments()

  useStatusSSE(deployments)

  const groups = useMemo(() => {
    if (!deployments) return []
    
    const map: Record<string, Deployment[]> = {}
    deployments.forEach(d => {
      // Group by source_url for git, or unique ID for uploads (unless we want to group uploads too)
      const key = d.source_url || `upload-${d.id}`
      if (!map[key]) map[key] = []
      map[key].push(d)
    })

    return Object.entries(map).map(([url, builds]) => {
      // Sort builds by creation date, latest first
      const sorted = builds.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      return {
        latest: sorted[0],
        all: sorted,
        url
      }
    }).sort((a, b) => new Date(b.latest.created_at).getTime() - new Date(a.latest.created_at).getTime())
  }, [deployments])

  if (isLoading) {
    return (
      <div className="bg-[#111216] border border-[#272a34] rounded-sm p-16 flex flex-col items-center justify-center text-[#797d8a]">
        <div className="mb-5 w-48">
           <SegmentedProgress active={true} />
        </div>
        <p className="font-mono text-xs tracking-wider uppercase">Loading state...</p>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="bg-[#111216] border border-[#fb7185]/30 rounded-sm p-12 text-center text-[#fb7185] font-mono text-sm">
        Failed to load instances.
      </div>
    )
  }

  return (
    <div className="bg-[#111216] border border-[#272a34] rounded-sm overflow-hidden flex flex-col h-full shadow-2xl">
      <div className="border-b border-[#272a34] bg-[#111216] px-4 py-2.5 flex justify-between items-center">
        <div className="flex items-center gap-2">
          <h2 className="font-mono text-[13px] font-semibold text-[#f5f6f9] tracking-wider uppercase">Services</h2>
          <span className="font-mono text-[11px] text-[#4a4d58] ml-1">[{groups.length}]</span>
        </div>
      </div>
      
      <div className="overflow-y-auto flex-1 bg-[#0a0b0d]">
        {groups.length > 0 ? (
          <div className="divide-y divide-[#272a34]">
            {groups.map((group) => (
              <DeploymentRow 
                key={group.latest.id} 
                deployment={group.latest} 
                history={group.all}
              />
            ))}
          </div>
        ) : (
          <div className="p-20 text-center text-[#797d8a]">
            <Server className="w-8 h-8 mx-auto mb-4 opacity-30" />
            <p className="font-mono text-xs text-[#797d8a] uppercase tracking-widest">— no services active —</p>
          </div>
        )}
      </div>
    </div>
  )
}
