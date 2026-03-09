import { useEffect, useState } from 'react'
import { api } from '../api'

const providerLabels = {
  siliconflow: '硅基流动 (siliconflow)',
  chatgpt: 'ChatGPT',
  claude: 'Claude',
  ollama: 'Ollama',
  vllm: 'vLLM'
}

const AVAILABLE_PROVIDER_ORDER = ['siliconflow', 'chatgpt']
const UNAVAILABLE_PROVIDER_ORDER = ['claude', 'ollama', 'vllm']

function normalizeProvider(name) {
  const raw = (name || '').trim().toLowerCase()
  if (raw === 'openai') return 'chatgpt'
  return raw
}

export function SettingsPage({ token }) {
  const [config, setConfig] = useState(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const res = await api.getConfig(token)
      setConfig(res)
      setError('')
    } catch (err) {
      setError(err.message)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  const update = async (provider) => {
    try {
      const res = await api.setConfig(token, provider)
      setConfig(res)
      setMessage(`已切换到 ${providerLabels[provider] || provider}。`)
      setError('')
    } catch (err) {
      setError(err.message)
      setMessage('')
    }
  }

  const activeProvider = normalizeProvider(config?.active_provider || '')
  const availableFromServer = new Set((config?.available || []).map((p) => normalizeProvider(p)))
  const availableProviders = AVAILABLE_PROVIDER_ORDER.filter((p) => availableFromServer.has(p))

  return (
    <section className="panel">
      <h2>System Configuration</h2>
      <p className="muted">选择新对话默认使用的模型提供商。</p>

      {config ? (
        <>
          <div className="provider-section">
            <h3>可用</h3>
            <div className="provider-grid">
              {availableProviders.map((provider) => (
                <button
                  key={provider}
                  className={activeProvider === provider ? 'provider-btn active' : 'provider-btn'}
                  onClick={() => update(provider)}
                >
                  {providerLabels[provider] || provider}
                </button>
              ))}
            </div>
          </div>

          <div className="provider-section">
            <h3>待扩展（不可用）</h3>
            <p className="muted provider-note">这些选项仅展示规划状态，当前版本不能切换使用。</p>
            <div className="provider-grid">
              {UNAVAILABLE_PROVIDER_ORDER.map((provider) => (
                <button key={provider} className="provider-btn provider-btn-disabled" disabled>
                  {providerLabels[provider] || provider}
                </button>
              ))}
            </div>
          </div>
        </>
      ) : (
        <p className="muted">Loading providers...</p>
      )}

      {message ? <div className="message ok">{message}</div> : null}
      {error ? <div className="message error">{error}</div> : null}
    </section>
  )
}
