import { apiClient } from '../client'
import type { Account, ApiKey } from '@/types'

export interface TicketDiagnostic {
  model: string
  status: 'normal' | 'degraded' | 'uncertain' | 'failed'
  reason?: string
  gateway_error_code?: string
  predicted_model?: string
  probability?: number
  parsed_number_count?: number
  http_status?: number
}

interface EnhancementOptions {
  timezones: string[]
  default_timezone: string
  models: string[]
  bank_commit: string
}

async function options() {
  const { data } = await apiClient.get<EnhancementOptions>('/admin/accounts/codex-enhancements/options')
  return data
}

export async function timezones() {
  const data = await options()
  return { timezones: data.timezones, default: data.default_timezone }
}

export async function detectTimezone(id: number) {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/request-timezone/detect`, {}, { timeout: 40000 })
  return data
}

export async function fingerprint() {
  const data = await options()
  return { commit: data.bank_commit, models: data.models }
}

// This build uses its audited embedded bank. Reloading never downloads executable
// code or an unreviewed replacement from the network.
export const refreshFingerprint = fingerprint

export async function ownKeys() {
  const keys: ApiKey[] = []
  for (let page = 1; ; page++) {
    const { data } = await apiClient.get<{ items: ApiKey[]; total: number }>('/keys', { params: { page, page_size: 100, status: 'active' } })
    keys.push(...data.items)
    if (keys.length >= data.total || !data.items.length) return keys
  }
}

export async function diagnose(id: number, apiKeyID: number, models: string[], signal?: AbortSignal) {
  const { data } = await apiClient.post<{ items: TicketDiagnostic[]; canceled: boolean; bank_commit: string }>(
    `/admin/accounts/${id}/model-diagnostic`, { api_key_id: apiKeyID, models }, { timeout: 370000, signal }
  )
  return data
}
