import { apiClient } from '@/api/client'

export type InvoiceType = 'normal' | 'special'
export type TitleType = 'personal' | 'company'
export type ApplicationStatus = 'pending' | 'processing' | 'issued' | 'rejected' | 'cancelled' | 'voided'
export type AccessMode = 'all' | 'allowlist'

/** Rules users see. Money values are decimal strings with two places. */
export interface InvoicePublicSettings {
  min_amount: string
  allow_special: boolean
  order_window_days: number
  item_name: string
  notice: string
}

export interface InvoiceAdminSettings extends InvoicePublicSettings {
  access_mode: AccessMode
  updated_at?: string | null
}

export interface InvoiceOverview {
  allowed: boolean
  settings?: InvoicePublicSettings
}

export interface InvoiceableOrder {
  id: number
  out_trade_no: string
  order_type: 'balance' | 'subscription' | string
  payment_type: string
  amount: string
  paid_at: string | null
}

export interface InvoiceableOrders {
  items: InvoiceableOrder[]
  total_amount: string
  truncated: boolean
}

export interface InvoiceTitleInput {
  title_type: TitleType
  name: string
  tax_no: string
  address: string
  phone: string
  bank_name: string
  bank_account: string
  email: string
  is_default: boolean
}

export interface InvoiceTitle extends InvoiceTitleInput {
  id: number
  created_at: string
  updated_at: string
}

export interface ApplicationOrder {
  order_id: number
  out_trade_no: string
  order_type: string
  amount: string
  paid_at: string | null
  /** Administrators only: the order was refunded after it was captured. */
  refunded?: boolean
}

export interface InvoiceApplication {
  id: number
  user_id: number
  user_email?: string
  invoice_type: InvoiceType
  title_type: TitleType
  title_name: string
  tax_no: string
  address: string
  phone: string
  bank_name: string
  bank_account: string
  email: string
  item_name: string
  amount: string
  order_count: number
  remark: string
  status: ApplicationStatus
  invoice_no: string
  reject_reason: string
  admin_note?: string
  has_file: boolean
  file_name?: string
  refund_alert?: boolean
  issued_at: string | null
  closed_at: string | null
  created_at: string
  updated_at: string
  orders?: ApplicationOrder[]
}

export interface ApplicationPage {
  items: InvoiceApplication[]
  total: number
  page: number
  page_size: number
  pages: number
  /** Administrators only: applications per status. */
  counts?: Partial<Record<ApplicationStatus, number>>
}

export interface AccessEntry {
  user_id: number
  email: string
  username: string
  note: string
  min_amount: string | null
  created_at: string
}

export interface ApplyInput {
  order_ids: number[]
  title_id: number
  invoice_type: InvoiceType
  remark: string
}

export interface ListQuery {
  page?: number
  page_size?: number
  status?: ApplicationStatus | ''
  q?: string
}

function query(params: ListQuery) {
  const out: Record<string, string | number> = {}
  if (params.page) out.page = params.page
  if (params.page_size) out.page_size = params.page_size
  if (params.status) out.status = params.status
  if (params.q?.trim()) out.q = params.q.trim()
  return out
}

/** Saves a downloaded PDF under a readable name. */
function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  document.body.appendChild(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function download(path: string, name: string) {
  const { data } = await apiClient.get(path, { responseType: 'blob' })
  saveBlob(data as Blob, name)
}

export function fileName(app: Pick<InvoiceApplication, 'id' | 'invoice_no'>): string {
  return app.invoice_no ? `发票-${app.invoice_no}.pdf` : `invoice-${app.id}.pdf`
}

export const invoiceAPI = {
  async overview(): Promise<InvoiceOverview> {
    const { data } = await apiClient.get('/invoices/overview')
    return data
  },
  async orders(): Promise<InvoiceableOrders> {
    const { data } = await apiClient.get('/invoices/orders')
    return data
  },
  async titles(): Promise<InvoiceTitle[]> {
    const { data } = await apiClient.get('/invoices/titles')
    return data.items ?? []
  },
  async saveTitle(input: InvoiceTitleInput, id?: number): Promise<InvoiceTitle> {
    const { data } = id
      ? await apiClient.put(`/invoices/titles/${id}`, input)
      : await apiClient.post('/invoices/titles', input)
    return data
  },
  async deleteTitle(id: number): Promise<void> {
    await apiClient.delete(`/invoices/titles/${id}`)
  },
  async applications(params: ListQuery = {}): Promise<ApplicationPage> {
    const { data } = await apiClient.get('/invoices/applications', { params: query(params) })
    return data
  },
  async application(id: number): Promise<InvoiceApplication> {
    const { data } = await apiClient.get(`/invoices/applications/${id}`)
    return data
  },
  async apply(input: ApplyInput): Promise<InvoiceApplication> {
    const { data } = await apiClient.post('/invoices/applications', input)
    return data
  },
  async cancel(id: number): Promise<void> {
    await apiClient.post(`/invoices/applications/${id}/cancel`)
  },
  download(app: InvoiceApplication): Promise<void> {
    return download(`/invoices/applications/${app.id}/file`, fileName(app))
  },
}

export const invoiceAdminAPI = {
  async settings(): Promise<InvoiceAdminSettings> {
    const { data } = await apiClient.get('/admin/invoices/settings')
    return data
  },
  async saveSettings(input: Omit<InvoiceAdminSettings, 'updated_at'>): Promise<InvoiceAdminSettings> {
    const { data } = await apiClient.put('/admin/invoices/settings', input)
    return data
  },
  async access(): Promise<AccessEntry[]> {
    const { data } = await apiClient.get('/admin/invoices/access')
    return data.items ?? []
  },
  async addAccess(users: string[], note: string): Promise<{ added: number; missing: string[] }> {
    const { data } = await apiClient.post('/admin/invoices/access', { users, note })
    return data
  },
  async removeAccess(userId: number): Promise<void> {
    await apiClient.delete(`/admin/invoices/access/${userId}`)
  },
  async updateAccess(userId: number, input: { note: string; min_amount: string | null }): Promise<void> {
    await apiClient.put(`/admin/invoices/access/${userId}`, input)
  },
  async applications(params: ListQuery = {}): Promise<ApplicationPage> {
    const { data } = await apiClient.get('/admin/invoices/applications', { params: query(params) })
    return data
  },
  async application(id: number): Promise<InvoiceApplication> {
    const { data } = await apiClient.get(`/admin/invoices/applications/${id}`)
    return data
  },
  async process(id: number): Promise<InvoiceApplication> {
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/process`)
    return data
  },
  async prepareMakeout(id: number): Promise<InvoiceApplication> {
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/prepare-makeout`)
    return data
  },
  async issue(id: number, invoiceNo: string, adminNote: string, file: File | null): Promise<InvoiceApplication> {
    const form = new FormData()
    form.append('invoice_no', invoiceNo)
    form.append('admin_note', adminNote)
    if (file) form.append('file', file)
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/issue`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  async uploadFile(id: number, file: File): Promise<InvoiceApplication> {
    const form = new FormData()
    form.append('file', file)
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/file`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  async reject(id: number, reason: string): Promise<InvoiceApplication> {
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/reject`, { reason })
    return data
  },
  async void(id: number, reason: string): Promise<InvoiceApplication> {
    const { data } = await apiClient.post(`/admin/invoices/applications/${id}/void`, { reason })
    return data
  },
  download(app: InvoiceApplication): Promise<void> {
    return download(`/admin/invoices/applications/${app.id}/file`, fileName(app))
  },
}
