/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

// 分站（代理）管理 API。对接后端 /api/agent/*（AdminAuth）。
// 这套表与普通 users/tokens 完全隔离，详见后端 controller/agent.go。

export interface Agent {
  id: number
  name: string
  agent_key: string
  domain: string
  status: number // 1 启用 2 禁用
  balance: number // 批发额度池，quota 单位
  used_quota: number // 累计消耗，quota 单位
  request_count: number
  group: string // 默认批发分组
  groups: string // 额外可用分组(逗号分隔)，空=与普通用户一致
  model_limits: string // 逗号分隔，空=不限
  remark: string
  created_at: number
  updated_at: number
}

export interface AgentLog {
  id: number
  agent_id: number
  model_name: string
  prompt_tokens: number
  completion_tokens: number
  quota: number
  channel_id: number
  use_time_seconds: number
  is_stream: boolean
  content: string
  created_at: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PageData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export type AgentFormData = {
  id?: number
  name: string
  group: string
  groups?: string
  domain?: string
  model_limits?: string
  remark?: string
  status?: number
}

export async function getAgents(
  p = 1,
  pageSize = 20
): Promise<ApiResponse<PageData<Agent>>> {
  const res = await api.get(`/api/agent/?p=${p}&page_size=${pageSize}`)
  return res.data
}

export async function createAgent(
  data: AgentFormData
): Promise<ApiResponse<Agent>> {
  const res = await api.post('/api/agent/', data)
  return res.data
}

export async function updateAgent(
  data: AgentFormData & { id: number }
): Promise<ApiResponse<Agent>> {
  const res = await api.put('/api/agent/', data)
  return res.data
}

export async function deleteAgent(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/agent/${id}`)
  return res.data
}

// quota 正数充值、负数扣减；单位为 quota。
export async function topupAgent(
  id: number,
  quota: number
): Promise<ApiResponse<Agent>> {
  const res = await api.post('/api/agent/topup', { id, quota })
  return res.data
}

export async function getAgentLogs(
  agentId: number,
  p = 1,
  pageSize = 20
): Promise<ApiResponse<PageData<AgentLog>>> {
  const res = await api.get(
    `/api/agent/logs?agent_id=${agentId}&p=${p}&page_size=${pageSize}`
  )
  return res.data
}
