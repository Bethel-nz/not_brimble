import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { Deployment, LogLinePayload, StatusPayload } from './types'

export function useStatusSSE(deployments: Deployment[] | undefined) {
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!deployments) return

    const activeSources = new Map<string, EventSource>()

    deployments.forEach((dep) => {
      if (['running', 'failed', 'stopped'].includes(dep.status)) return

      if (activeSources.has(dep.id)) return

      const source = new EventSource(`/api/deployments/${dep.id}/status`)
      activeSources.set(dep.id, source)

      source.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data) as StatusPayload

          queryClient.setQueryData<Deployment[]>(['deployments'], (old) => {
            if (!old) return old
            return old.map((d) => {
              if (d.id === dep.id) {
                return {
                  ...d,
                  status: payload.status,
                  subdomain: payload.subdomain,
                  image_tag: payload.image_tag,
                }
              }
              return d
            })
          })

          if (['running', 'failed', 'stopped'].includes(payload.status)) {
            source.close()
            activeSources.delete(dep.id)
          }
        } catch (err) {
          console.error('Failed to parse status SSE', err)
        }
      }

      source.onerror = () => {
        source.close()
        activeSources.delete(dep.id)
      }
    })

    return () => {
      activeSources.forEach((source) => source.close())
      activeSources.clear()
    }
  }, [deployments, queryClient])
}

export function useLogsSSE(deploymentId: string, isOpen: boolean) {
  const [logs, setLogs] = useState<LogLinePayload[]>([])
  const [isStreaming, setIsStreaming] = useState(false)

  useEffect(() => {
    if (!isOpen) return

    setLogs([])
    setIsStreaming(true)

    const source = new EventSource(`/api/deployments/${deploymentId}/logs`)

    source.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as LogLinePayload
        setLogs((prev) => [...prev, payload])
      } catch (err) {
        setLogs((prev) => [...prev, { id: Date.now(), stream: 'stdout', line: event.data }])
      }
    }

    source.onerror = () => {
      setIsStreaming(false)
      source.close()
    }

    return () => {
      setIsStreaming(false)
      source.close()
    }
  }, [deploymentId, isOpen])

  return { logs, isStreaming }
}
