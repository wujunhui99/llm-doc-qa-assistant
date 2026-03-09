const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

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

async function streamRequest(path, { method = 'POST', token, body, onEvent } = {}) {
  const headers = { Accept: 'text/event-stream' }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  if (body) {
    headers['Content-Type'] = 'application/json'
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined
  })

  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    const msg = data?.error?.message || `Request failed (${res.status})`
    throw new Error(msg)
  }
  if (!res.body) {
    throw new Error('Streaming response body is empty')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const emitFrame = (frame) => {
    const lines = frame.split('\n')
    let eventName = 'message'
    const dataLines = []
    for (const rawLine of lines) {
      const line = rawLine.trimEnd()
      if (!line) continue
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trim())
      }
    }
    const dataText = dataLines.join('\n')
    let payload = {}
    if (dataText) {
      try {
        payload = JSON.parse(dataText)
      } catch {
        payload = { raw: dataText }
      }
    }
    if (onEvent) onEvent(eventName, payload)
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let idx = buffer.indexOf('\n\n')
    while (idx >= 0) {
      const frame = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      if (frame.trim()) emitFrame(frame)
      idx = buffer.indexOf('\n\n')
    }
  }

  if (buffer.trim()) {
    emitFrame(buffer)
  }
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
  listTurns: (token, threadId) => request(`/api/threads/${threadId}/turns`, { token }),
  createTurn: (token, threadId, payload) => request(`/api/threads/${threadId}/turns`, { method: 'POST', token, body: payload }),
  createTurnStream: (token, threadId, payload, onEvent) =>
    streamRequest(`/api/threads/${threadId}/turns/stream`, { method: 'POST', token, body: payload, onEvent }),

  getConfig: (token) => request('/api/config', { token }),
  setConfig: (token, activeProvider) => request('/api/config', { method: 'PUT', token, body: { active_provider: activeProvider } })
}

export { API_BASE_URL }
