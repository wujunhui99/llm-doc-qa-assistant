const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

async function request(path, { method = 'GET', token, body, isFormData = false } = {}) {
  const headers = {}
  if (!isFormData) {
    headers['Content-Type'] = 'application/json'
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers,
    body: body ? (isFormData ? body : JSON.stringify(body)) : undefined
  })

  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg = data?.error?.message || `Request failed (${res.status})`
    throw new Error(msg)
  }
  return data
}

export const api = {
  register: (email, password) => request('/api/auth/register', { method: 'POST', body: { email, password } }),
  login: (email, password) => request('/api/auth/login', { method: 'POST', body: { email, password } }),
  me: (token) => request('/api/auth/me', { token }),
  logout: (token) => request('/api/auth/logout', { method: 'POST', token }),

  listDocuments: (token) => request('/api/documents', { token }),
  uploadDocument: (token, file) => {
    const form = new FormData()
    form.append('file', file)
    return request('/api/documents/upload', { method: 'POST', token, body: form, isFormData: true })
  },
  downloadDocument: async (token, docId) => {
    const res = await fetch(`${API_BASE_URL}/api/documents/${docId}/download`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`
      }
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      const msg = data?.error?.message || `Download failed (${res.status})`
      throw new Error(msg)
    }
    return await res.blob()
  },
  deleteDocument: (token, docId) => request(`/api/documents/${docId}?confirm=true`, { method: 'DELETE', token }),

  listThreads: (token) => request('/api/threads', { token }),
  createThread: (token, title) => request('/api/threads', { method: 'POST', token, body: { title } }),
  createTurn: (token, threadId, payload) => request(`/api/threads/${threadId}/turns`, { method: 'POST', token, body: payload }),

  getConfig: (token) => request('/api/config', { token }),
  setConfig: (token, activeProvider) => request('/api/config', { method: 'PUT', token, body: { active_provider: activeProvider } })
}

export { API_BASE_URL }
