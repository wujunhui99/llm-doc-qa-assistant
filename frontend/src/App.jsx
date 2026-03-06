import { useEffect, useMemo, useState } from 'react'
import { api } from './api'
import { AuthPage } from './pages/AuthPage'
import { DocumentsPage } from './pages/DocumentsPage'
import { QAPage } from './pages/QAPage'
import { SettingsPage } from './pages/SettingsPage'

const TABS = [
  { id: 'documents', label: 'Documents' },
  { id: 'qa', label: 'Agent QA' },
  { id: 'settings', label: 'Settings' }
]

export default function App() {
  const [token, setToken] = useState(() => localStorage.getItem('qa_token') || '')
  const [user, setUser] = useState(null)
  const [activeTab, setActiveTab] = useState('documents')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!token) {
      setUser(null)
      return
    }
    api.me(token)
      .then((res) => {
        setUser(res.user)
        setError('')
      })
      .catch((err) => {
        setToken('')
        localStorage.removeItem('qa_token')
        setError(err.message)
      })
  }, [token])

  const onLoginSuccess = (nextToken, nextUser) => {
    setToken(nextToken)
    setUser(nextUser)
    localStorage.setItem('qa_token', nextToken)
    setError('')
  }

  const handleLogout = async () => {
    try {
      await api.logout(token)
    } catch {
      // Keep local logout deterministic even if backend session already expired.
    }
    setToken('')
    setUser(null)
    localStorage.removeItem('qa_token')
  }

  const page = useMemo(() => {
    if (!token || !user) {
      return <AuthPage onLoginSuccess={onLoginSuccess} />
    }
    if (activeTab === 'documents') {
      return <DocumentsPage token={token} user={user} />
    }
    if (activeTab === 'qa') {
      return <QAPage token={token} user={user} />
    }
    return <SettingsPage token={token} />
  }, [activeTab, token, user])

  return (
    <div className="app-shell">
      <div className="backdrop-glow" />
      <header className="topbar">
        <div>
          <h1>Smart Document QA Assistant</h1>
          <p>Grounded answers with citations and strict per-user isolation.</p>
        </div>
        {user ? (
          <div className="topbar-user">
            <span>{user.email}</span>
            <button className="ghost-btn" onClick={handleLogout}>Logout</button>
          </div>
        ) : null}
      </header>

      {user ? (
        <nav className="tabs">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              className={tab.id === activeTab ? 'tab tab-active' : 'tab'}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      ) : null}

      {error ? <div className="message error">{error}</div> : null}
      <main className="content">{page}</main>
    </div>
  )
}
