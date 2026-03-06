import { useEffect, useState } from 'react'
import { api } from '../api'

const providerLabels = {
  siliconflow: '硅基流动 (siliconflow)',
  mock: 'Mock',
  openai: 'OpenAI',
  claude: 'Claude',
  local: 'Local'
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
      setMessage(`Active provider switched to ${provider}.`)
      setError('')
    } catch (err) {
      setError(err.message)
      setMessage('')
    }
  }

  return (
    <section className="panel">
      <h2>System Configuration</h2>
      <p className="muted">Select the active provider adapter for new QA turns.</p>

      {config ? (
        <div className="provider-grid">
          {config.available?.map((provider) => (
            <button
              key={provider}
              className={config.active_provider === provider ? 'provider-btn active' : 'provider-btn'}
              onClick={() => update(provider)}
            >
              {providerLabels[provider] || provider}
            </button>
          ))}
        </div>
      ) : (
        <p className="muted">Loading providers...</p>
      )}

      {message ? <div className="message ok">{message}</div> : null}
      {error ? <div className="message error">{error}</div> : null}
    </section>
  )
}
