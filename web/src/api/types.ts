export type DeploymentStatus =
  | 'pending'
  | 'building'
  | 'built'
  | 'deploying'
  | 'running'
  | 'failed'
  | 'stopped'

export interface Deployment {
  id: string
  name: string
  source_type: 'git' | 'upload'
  source_url: string
  image_tag: string
  container_id: string
  subdomain: string
  caddy_route_id: string
  status: DeploymentStatus
  created_at: string
  updated_at: string
}

export interface LogLinePayload {
  id: number
  stream: 'stdout' | 'stderr'
  line: string
}

export interface StatusPayload {
  status: DeploymentStatus
  subdomain: string
  image_tag: string
}
