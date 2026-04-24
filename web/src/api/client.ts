import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { Deployment } from './types'

const BASE_URL = '/api'

export function useDeployments() {
  return useQuery<Deployment[]>({
    queryKey: ['deployments'],
    queryFn: async () => {
      const res = await fetch(`${BASE_URL}/deployments`)
      if (!res.ok) throw new Error('Failed to fetch deployments')
      return res.json()
    },
  })
}

export function useCreateDeployment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (data: { sourceType: 'git'; sourceUrl: string } | { sourceType: 'upload'; file: File }) => {
      if (data.sourceType === 'git') {
        const res = await fetch(`${BASE_URL}/deployments`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'git', source_url: data.sourceUrl }),
        })
        if (!res.ok) throw new Error('Failed to create deployment')
        return res.json()
      } else {
        const formData = new FormData()
        formData.append('source_type', 'upload')
        formData.append('file', data.file)
        const res = await fetch(`${BASE_URL}/deployments`, {
          method: 'POST',
          body: formData,
        })
        if (!res.ok) throw new Error('Failed to create deployment')
        return res.json()
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
    },
  })
}

export function useDeleteDeployment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`${BASE_URL}/deployments/${id}`, {
        method: 'DELETE',
      })
      if (!res.ok) throw new Error('Failed to delete deployment')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
    },
  })
}

export function useRedeployDeployment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`${BASE_URL}/deployments/${id}/redeploy`, {
        method: 'POST',
      })
      if (!res.ok) throw new Error('Failed to redeploy')
      return res.json() as Promise<Deployment>
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
    },
  })
}
