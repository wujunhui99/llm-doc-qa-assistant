import { useEffect, useMemo, useState } from 'react'
import { api } from '../api'

export function QAPage({ token }) {
  const [documents, setDocuments] = useState([])
  const [threads, setThreads] = useState([])
  const [activeThreadId, setActiveThreadId] = useState('')
  const [scopeType, setScopeType] = useState('all')
  const [docIDs, setDocIDs] = useState([])
  const [message, setMessage] = useState('')
  const [turns, setTurns] = useState([])
  const [streamingTurn, setStreamingTurn] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const activeThread = useMemo(() => threads.find((t) => t.id === activeThreadId) || null, [threads, activeThreadId])

  const bootstrap = async () => {
    try {
      const [docRes, threadRes] = await Promise.all([api.listDocuments(token), api.listThreads(token)])
      setDocuments(docRes.documents || [])
      const existingThreads = threadRes.threads || []
      setThreads(existingThreads)
      if (existingThreads.length) {
        setActiveThreadId(existingThreads[0].id)
      }
      setError('')
    } catch (err) {
      setError(err.message)
    }
  }

  useEffect(() => {
    bootstrap()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  const createThread = async () => {
    try {
      const res = await api.createThread(token, `Session ${new Date().toLocaleTimeString()}`)
      const next = [res.thread, ...threads]
      setThreads(next)
      setActiveThreadId(res.thread.id)
      setTurns([])
    } catch (err) {
      setError(err.message)
    }
  }

  const onDocScopeChange = (id) => {
    setDocIDs((prev) => (prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]))
  }

  const ask = async (e) => {
    e.preventDefault()
    if (!activeThreadId) {
      setError('Create or choose a thread first.')
      return
    }
    if (!message.trim()) {
      setError('Please type your question.')
      return
    }
    if (scopeType === 'doc' && docIDs.length === 0) {
      setError('Doc scope requires selecting at least one document.')
      return
    }

    const payload = {
      message,
      scope_type: scopeType,
      scope_doc_ids: scopeType === 'doc' ? docIDs : []
    }

    setLoading(true)
    setError('')
    try {
      const draft = {
        turn: {
          id: `local_${Date.now()}`,
          question: message.trim(),
          answer: '',
          scope_type: scopeType
        },
        citations: [],
        items: []
      }
      let streamError = ''
      setStreamingTurn(draft)

      await api.createTurnStream(token, activeThreadId, payload, (event, data) => {
        if (event === 'error') {
          streamError = data?.message || 'Streaming failed'
          return
        }

        const payloadObj = data?.payload || {}
        if (data?.turn_id) {
          draft.turn.id = data.turn_id
        }
        if (event === 'message') {
          draft.turn.question = payloadObj.question || draft.turn.question
          draft.turn.scope_type = payloadObj.scope_type || draft.turn.scope_type
        } else if (event === 'retrieval') {
          draft.citations = Array.isArray(payloadObj.citations) ? payloadObj.citations : []
        } else if (event === 'delta') {
          const delta = payloadObj.delta || ''
          if (delta) {
            draft.turn.answer = `${draft.turn.answer}${delta}`
          }
        } else if (event === 'final') {
          draft.turn.answer = payloadObj.answer || draft.turn.answer
          draft.citations = Array.isArray(payloadObj.citations) ? payloadObj.citations : draft.citations
        }
        setStreamingTurn({
          ...draft,
          turn: { ...draft.turn },
          citations: [...draft.citations]
        })
      })

      if (streamError) {
        throw new Error(streamError)
      }

      setTurns((prev) => [
        {
          ...draft,
          turn: { ...draft.turn },
          citations: [...draft.citations],
          items: []
        },
        ...prev
      ])
      setStreamingTurn(null)
      setMessage('')
    } catch (err) {
      setStreamingTurn(null)
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="panel">
      <h2>Agent QA</h2>

      <div className="qa-toolbar">
        <button onClick={createThread}>New Thread</button>
        <select value={activeThreadId} onChange={(e) => setActiveThreadId(e.target.value)}>
          <option value="">Select Thread</option>
          {threads.map((thread) => (
            <option key={thread.id} value={thread.id}>{thread.title}</option>
          ))}
        </select>
        {activeThread ? <span className="muted">Active: {activeThread.title}</span> : null}
      </div>

      <form onSubmit={ask} className="qa-form">
        <label>
          Scope
          <select value={scopeType} onChange={(e) => setScopeType(e.target.value)}>
            <option value="all">@all</option>
            <option value="doc">@doc</option>
          </select>
        </label>

        {scopeType === 'doc' ? (
          <div className="doc-scope-grid">
            {documents.map((doc) => (
              <label key={doc.id} className="scope-check">
                <input
                  type="checkbox"
                  checked={docIDs.includes(doc.id)}
                  onChange={() => onDocScopeChange(doc.id)}
                />
                <span>{doc.name}</span>
              </label>
            ))}
            {documents.length === 0 ? <p className="muted">Upload documents first.</p> : null}
          </div>
        ) : null}

        <textarea
          rows={4}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Ask a grounded question..."
        />

        <button type="submit" disabled={loading}>{loading ? 'Thinking...' : 'Submit Turn'}</button>
      </form>

      {error ? <div className="message error">{error}</div> : null}

      <div className="turn-list">
        {!streamingTurn && turns.length === 0 ? <p className="muted">No turns yet for this session.</p> : null}
        {streamingTurn ? (
          <article className="turn-card" key={streamingTurn.turn.id}>
            <div className="turn-head">
              <span>Q: {streamingTurn.turn.question}</span>
              <span className="status status-ready">streaming</span>
            </div>
            <p className="turn-answer">{streamingTurn.turn.answer || '...'}</p>

            <h4>Citations</h4>
            {streamingTurn.citations?.length ? (
              <ul>
                {streamingTurn.citations.map((c) => (
                  <li key={c.chunk_id}>
                    <strong>{c.doc_name}</strong>#{c.chunk_index}: {c.excerpt}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">No citation found in this turn.</p>
            )}
          </article>
        ) : null}
        {turns.map((entry) => (
          <article className="turn-card" key={entry.turn.id}>
            <div className="turn-head">
              <span>Q: {entry.turn.question}</span>
              <span className="status status-ready">{entry.turn.scope_type}</span>
            </div>
            <p className="turn-answer">{entry.turn.answer}</p>

            <h4>Citations</h4>
            {entry.citations?.length ? (
              <ul>
                {entry.citations.map((c) => (
                  <li key={c.chunk_id}>
                    <strong>{c.doc_name}</strong>#{c.chunk_index}: {c.excerpt}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">No citation found in this turn.</p>
            )}
          </article>
        ))}
      </div>
    </section>
  )
}
