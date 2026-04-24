import { useState } from 'react'
import { motion } from 'motion/react'
import { useCreateDeployment } from '../api/client'

export function CreateForm() {
  const [tab, setTab] = useState<'git' | 'upload'>('git')
  const [gitUrl, setGitUrl] = useState('')
  const [file, setFile] = useState<File | null>(null)
  
  const createMutation = useCreateDeployment()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (tab === 'git' && gitUrl) {
      createMutation.mutate(
        { sourceType: 'git', sourceUrl: gitUrl },
        { onSuccess: () => setGitUrl('') }
      )
    } else if (tab === 'upload' && file) {
      createMutation.mutate(
        { sourceType: 'upload', file },
        { onSuccess: () => setFile(null) }
      )
    }
  }

  const isPending = createMutation.isPending

  return (
    <div className="bg-[#111216] border border-[#272a34] rounded-sm p-4">
      <div className="flex items-center gap-3 mb-4">
        <div className="flex-1 min-w-0">
          <h3 className="font-mono text-[12px] text-[#f5f6f9] font-semibold uppercase tracking-wider">New Deployment</h3>
          <p className="font-mono text-[10.5px] text-[#797d8a] mt-0.5">Railpack builds a container · Caddy serves it at a subdomain</p>
        </div>
        <div className="flex gap-0.5 p-0.5 bg-[#0a0b0d] border border-[#272a34] rounded-sm">
          <button
            type="button"
            onClick={() => setTab('git')}
            className={`font-mono text-[10.5px] px-2.5 py-1 rounded-sm transition ${
              tab === 'git' ? 'bg-[#1e2028] text-[#f5f6f9]' : 'text-[#797d8a] hover:text-[#d4d6dd]'
            }`}
          >
            Git
          </button>
          <button
            type="button"
            onClick={() => setTab('upload')}
            className={`font-mono text-[10.5px] px-2.5 py-1 rounded-sm transition ${
              tab === 'upload' ? 'bg-[#1e2028] text-[#f5f6f9]' : 'text-[#797d8a] hover:text-[#d4d6dd]'
            }`}
          >
            Upload
          </button>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="flex items-stretch gap-2">
        {tab === 'git' ? (
          <div className="flex-1 flex items-center gap-2 bg-[#0a0b0d] border border-[#272a34] focus-within:border-[#4ade80]/50 rounded-sm px-3 transition">
            <span className="font-mono text-[#4ade80]/80">›</span>
            <input
              type="text"
              value={gitUrl}
              onChange={e => setGitUrl(e.target.value)}
              placeholder="github.com/acme/my-service"
              className="flex-1 bg-transparent font-mono text-[12.5px] text-[#f5f6f9] placeholder:text-[#4a4d58] py-2 focus:outline-none"
            />
          </div>
        ) : (
          <label className="flex-1 flex items-center gap-2 bg-[#0a0b0d] border border-dashed border-[#272a34] hover:border-[#4ade80]/50 rounded-sm px-3 py-2 cursor-pointer transition">
            <span className={`font-mono text-[12.5px] ${file ? 'text-[#f5f6f9]' : 'text-[#4a4d58]'}`}>
              {file?.name || 'choose .tar.gz / .zip'}
            </span>
            <input type="file" accept=".tar.gz,.tgz,.zip,application/gzip,application/x-gzip,application/x-tar,application/zip" onChange={e => setFile(e.target.files?.[0] || null)} className="hidden"/>
          </label>
        )}
        
        <button
          type="submit"
          disabled={isPending || (tab === 'git' && !gitUrl) || (tab === 'upload' && !file)}
          className="font-mono text-[12px] font-semibold text-[#0a0b0d] bg-[#4ade80] hover:bg-[#22c55e] disabled:bg-[#1e2028] disabled:text-[#4a4d58] disabled:cursor-not-allowed px-4 rounded-sm transition inline-flex items-center gap-2"
        >
          {isPending ? (
            <span className="animate-pulse font-bold">QUEUEING...</span>
          ) : (
            <span className="font-bold uppercase tracking-widest">▲ deploy</span>
          )}
        </button>
      </form>
    </div>
  )
}
