import { apiFetch } from './client'

const BASE = '/api/v1/tenants'

export interface PIARule {
  label: string
  type: string
  pattern: string
  action: string
}

export interface PIIConfig {
  enabled: boolean
  default_action: string
  rules: PIARule[]
}

export interface DictionaryItem {
  name: string
  entries: any[]
  match_mode: string
}

export type RetentionMode = 'full' | 'meta' | 'none'

export interface TenantListItem {
  slug: string
  name: string
  auth_header: string
  api_keys: string[]
  pii_config?: PIIConfig
  retention_mode?: RetentionMode
  created_at: string
  updated_at: string
}

export interface TenantResponse {
  slug: string
  name: string
  auth_header: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
  retention_mode?: RetentionMode
  created_at: string
  updated_at: string
}

export interface CreateTenantRequest {
  slug: string
  name: string
  auth_header?: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
  retention_mode?: RetentionMode
}

export interface UpdateTenantRequest {
  name: string
  auth_header?: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
  retention_mode?: RetentionMode
}

export interface DictionaryRequest {
  dictionaries: DictionaryItem[]
}

export interface DictionaryResponse {
  dictionaries: DictionaryItem[]
}

export interface ErrorResponse {
  error: string
  code: string
  details?: { field: string; message: string }[]
}

export async function listTenants(): Promise<TenantListItem[]> {
  return apiFetch<TenantListItem[]>(BASE)
}

export async function getTenant(slug: string): Promise<TenantResponse> {
  return apiFetch<TenantResponse>(`${BASE}/${encodeURIComponent(slug)}`)
}

export async function createTenant(req: CreateTenantRequest): Promise<TenantResponse> {
  return apiFetch<TenantResponse>(BASE, {
    method: 'POST',
    body: req,
  })
}

export async function updateTenant(slug: string, req: UpdateTenantRequest): Promise<TenantResponse> {
  return apiFetch<TenantResponse>(`${BASE}/${encodeURIComponent(slug)}`, {
    method: 'PUT',
    body: req,
  })
}

export async function deleteTenant(slug: string): Promise<void> {
  await apiFetch<void>(`${BASE}/${encodeURIComponent(slug)}`, {
    method: 'DELETE',
  })
}

export async function updateDictionaries(slug: string, req: DictionaryRequest): Promise<DictionaryResponse> {
  return apiFetch<DictionaryResponse>(`${BASE}/${encodeURIComponent(slug)}/dictionaries`, {
    method: 'PUT',
    body: req,
  })
}

export interface ComplianceRule {
  detector_type: string
  reaction: string
  masking: boolean
}

export interface ComplianceApplyResult {
  pack_key: string
  rules: ComplianceRule[]
}

export interface ComplianceReportRule {
  detector_type: string
  expected_reaction: string
  actual_reaction?: string
  masking: boolean
  status: 'active' | 'deviated' | 'missing'
}

export interface ComplianceReport {
  pack_key: string
  rules: ComplianceReportRule[]
}

export const COMPLIANCE_PACKS = ['HIPAA', 'PCI DSS', 'GDPR', 'Legal'] as const

export async function applyCompliancePack(slug: string, packKey: string): Promise<ComplianceApplyResult> {
  return apiFetch<ComplianceApplyResult>(`${BASE}/${encodeURIComponent(slug)}/compliance/apply`, {
    method: 'POST',
    body: { pack_key: packKey },
  })
}

export async function getComplianceReport(slug: string, packKey: string): Promise<ComplianceReport> {
  return apiFetch<ComplianceReport>(
    `${BASE}/${encodeURIComponent(slug)}/compliance/report?pack=${encodeURIComponent(packKey)}`,
  )
}
