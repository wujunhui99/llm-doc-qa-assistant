import { useState } from 'react'
import { api } from '../api'

export function AuthPage({ onLoginSuccess }) {
  const [mode, setMode] = useState('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      if (mode === 'register') {
        await api.register(email, password)
      }
      const loginRes = await api.login(email, password)
      onLoginSuccess(loginRes.token, loginRes.user)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="panel auth-panel">
      <h2>{mode === 'login' ? 'Login' : 'Register'}</h2>
      <p className="muted">Create an account and start grounding answers in your own documents.</p>

      <form onSubmit={submit} className="auth-form">
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>

        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={8}
            required
          />
        </label>

        {error ? <div className="message error">{error}</div> : null}

        <button type="submit" disabled={loading}>
          {loading ? 'Processing...' : mode === 'login' ? 'Login' : 'Register'}
        </button>
      </form>

      <button
        className="link-btn"
        onClick={() => setMode(mode === 'login' ? 'register' : 'login')}
      >
        {mode === 'login' ? 'Need an account? Register' : 'Have an account? Login'}
      </button>
    </section>
  )
}
