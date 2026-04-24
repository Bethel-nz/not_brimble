import { createFileRoute } from '@tanstack/react-router'
import { CreateForm } from '../components/CreateForm'
import { DeploymentList } from '../components/DeploymentList'

export const Route = createFileRoute('/')({ component: Dashboard })

function Dashboard() {
  return (
    <div className="min-h-screen bg-[#0a0b0d] text-[#d4d6dd] font-sans selection:bg-[#4ade80]/20 flex flex-col items-center py-12 px-6">
      <main className="w-full max-w-[1000px] flex flex-col gap-10">
        
        {/* Branding Ribbon */}
        <div className="flex items-center justify-between">
           <div className="flex items-center gap-3">
             <div className="w-7 h-7 bg-[#4ade80]/10 border border-[#4ade80]/30 flex items-center justify-center">
               <span className="font-mono text-[#4ade80] text-[15px] font-bold">▲</span>
             </div>
             <h1 className="font-mono text-[16px] text-[#f5f6f9] font-bold tracking-tight uppercase">not brimble</h1>
           </div>
           
           <div className="font-mono text-[10px] text-[#565665] flex items-center gap-4 font-bold tracking-[0.2em] uppercase">
             <span className="flex items-center gap-1.5"><span className="w-1.5 h-1.5 rounded-full bg-[#4ade80] dot-pulse" /> session active</span>
             <span className="text-[#272a34]">|</span>
             <span>production cluster</span>
           </div>
        </div>
        
        <div className="flex flex-col gap-8">
           <CreateForm />
           <DeploymentList />
        </div>

        <footer className="mt-8 border-t border-[#272a34] pt-4 flex justify-between mono text-[9px] text-[#565665] uppercase tracking-widest font-bold">
           <span>v0.4.5-stable</span>
           <span>cluster: us-east-1 · via buildkitd</span>
        </footer>
        
      </main>
    </div>
  )
}
