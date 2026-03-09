import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../api'

function stripDocMentions(text) {
  return text.replace(/@doc\(([^)]+)\)/g, ' ').replace(/\s+/g, ' ').trim()
}

function turnCacheStorageKey(token) {
  const scope = (token || '').slice(0, 24) || 'anon'
  return `qa_turn_cache_v1:${scope}`
}

function activeThreadStorageKey(token) {
  const scope = (token || '').slice(0, 24) || 'anon'
  return `qa_active_thread_v1:${scope}`
}

function loadCachedTurns(token, threadId) {
  if (!threadId) return null
  try {
    const raw = localStorage.getItem(turnCacheStorageKey(token))
    if (!raw) return null
    const parsed = JSON.parse(raw)
    const rows = parsed?.[threadId]
    return Array.isArray(rows) ? rows : null
  } catch {
    return null
  }
}

function saveCachedTurns(token, threadId, turns) {
  if (!threadId) return
  try {
    const key = turnCacheStorageKey(token)
    const raw = localStorage.getItem(key)
    const parsed = raw ? JSON.parse(raw) : {}
    parsed[threadId] = Array.isArray(turns) ? turns.slice(0, 100) : []
    localStorage.setItem(key, JSON.stringify(parsed))
  } catch {
    // Ignore cache write errors.
  }
}

function loadActiveThread(token) {
  try {
    return localStorage.getItem(activeThreadStorageKey(token)) || ''
  } catch {
    return ''
  }
}

function saveActiveThread(token, threadId) {
  try {
    const key = activeThreadStorageKey(token)
    if (threadId) {
      localStorage.setItem(key, threadId)
    } else {
      localStorage.removeItem(key)
    }
  } catch {
    // Ignore cache write errors.
  }
}

function normalizeTurnRows(rows) {
  const out = []
  const list = Array.isArray(rows) ? rows : []
  for (const row of list) {
    const turn = row?.turn && typeof row.turn === 'object' ? row.turn : null
    if (!turn) continue
    const items = Array.isArray(row?.items) ? row.items : []
    let citations = []
    let retrievalDecision = null
    for (const item of items) {
      const type = item?.item_type || ''
      const payload = item?.payload && typeof item.payload === 'object' ? item.payload : {}
      if (type === 'retrieval_decision') {
        retrievalDecision = payload
      }
      if ((type === 'retrieval' || type === 'final') && Array.isArray(payload.citations)) {
        citations = payload.citations
      }
    }
    out.push({
      turn,
      citations,
      retrieval_decision: retrievalDecision,
      items
    })
  }
  return out
}

export function QAPage({ token }) {
  const [documents, setDocuments] = useState([])
  const [threads, setThreads] = useState([])
  const [activeThreadId, setActiveThreadId] = useState('')
  const [message, setMessage] = useState('')
  const [turns, setTurns] = useState([])
  const [streamingTurn, setStreamingTurn] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [mentionTarget, setMentionTarget] = useState(null)
  const [mentionIndex, setMentionIndex] = useState(0)
  const textAreaRef = useRef(null)

  const activeThread = useMemo(() => threads.find((t) => t.id === activeThreadId) || null, [threads, activeThreadId])
  const docByID = useMemo(() => {
    const out = new Map()
    documents.forEach((doc) => out.set(doc.id, doc))
    return out
  }, [documents])
  const docIDByName = useMemo(() => {
    const out = new Map()
    documents.forEach((doc) => out.set(doc.name.toLowerCase(), doc.id))
    return out
  }, [documents])

  const mentionedDocIDs = useMemo(() => {
    const ids = []
    for (const match of message.matchAll(/@doc\(([^)]+)\)/g)) {
      const raw = (match?.[1] || '').trim()
      if (!raw) continue
      let docID = ''
      if (docByID.has(raw)) {
        docID = raw
      } else {
        docID = docIDByName.get(raw.toLowerCase()) || ''
      }
      if (docID && !ids.includes(docID)) {
        ids.push(docID)
      }
    }
    return ids
  }, [message, docByID, docIDByName])

  const mentionedDocs = useMemo(
    () => mentionedDocIDs.map((id) => docByID.get(id)).filter(Boolean),
    [mentionedDocIDs, docByID]
  )

  const mentionSuggestions = useMemo(() => {
    if (!mentionTarget) return []
    const q = (mentionTarget.query || '').trim().toLowerCase()
    return documents
      .filter((doc) => !q || doc.name.toLowerCase().includes(q))
      .slice(0, 8)
  }, [mentionTarget, documents])

  const bootstrap = async () => {
    try {
      const [docRes, threadRes] = await Promise.all([api.listDocuments(token), api.listThreads(token)])
      setDocuments(docRes.documents || [])
      const existingThreads = threadRes.threads || []
      setThreads(existingThreads)
      const cachedActive = loadActiveThread(token)
      const hasCached = cachedActive && existingThreads.some((thread) => thread.id === cachedActive)
      if (hasCached) {
        setActiveThreadId(cachedActive)
      } else if (existingThreads.length) {
        setActiveThreadId(existingThreads[0].id)
      } else {
        setActiveThreadId('')
        setTurns([])
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

  useEffect(() => {
    saveActiveThread(token, activeThreadId)
  }, [token, activeThreadId])

  useEffect(() => {
    saveCachedTurns(token, activeThreadId, turns)
  }, [token, activeThreadId, turns])

  useEffect(() => {
    let cancelled = false
    const loadTurns = async () => {
      if (!activeThreadId) {
        setTurns([])
        setStreamingTurn(null)
        return
      }

      const cached = loadCachedTurns(token, activeThreadId)
      if (cached) {
        setTurns(cached)
      } else {
        setTurns([])
      }
      setStreamingTurn(null)

      try {
        const res = await api.listTurns(token, activeThreadId)
        if (cancelled) return
        const normalized = normalizeTurnRows(res.turns || [])
        setTurns(normalized)
        saveCachedTurns(token, activeThreadId, normalized)
        setError('')
      } catch (err) {
        if (cancelled) return
        if (!cached) {
          setTurns([])
        }
        setError(err.message)
      }
    }

    loadTurns()
    return () => {
      cancelled = true
    }
  }, [token, activeThreadId])

  const createThread = async () => {
    try {
      const res = await api.createThread(token, `Session ${new Date().toLocaleTimeString()}`)
      const next = [res.thread, ...threads]
      setThreads(next)
      setActiveThreadId(res.thread.id)
      setTurns([])
      saveCachedTurns(token, res.thread.id, [])
    } catch (err) {
      setError(err.message)
    }
  }

  const syncMentionTarget = (nextMessage, cursorPos) => {
    const cursor = typeof cursorPos === 'number' ? cursorPos : nextMessage.length
    const before = nextMessage.slice(0, cursor)
    const match = before.match(/@([^\s@()]*)$/)
    if (!match) {
      if (mentionTarget !== null) setMentionTarget(null)
      return
    }
    const atIdx = before.lastIndexOf('@')
    if (atIdx < 0) {
      if (mentionTarget !== null) setMentionTarget(null)
      return
    }
    setMentionTarget({ query: match[1] || '', start: atIdx, end: cursor })
    setMentionIndex(0)
  }

  const applyMention = (doc) => {
    if (!mentionTarget) return
    const tokenText = `@doc(${doc.name})`
    const nextMessage = `${message.slice(0, mentionTarget.start)}${tokenText} ${message.slice(mentionTarget.end)}`
    const nextCursor = mentionTarget.start + tokenText.length + 1
    setMessage(nextMessage)
    setMentionTarget(null)
    setMentionIndex(0)
    requestAnimationFrame(() => {
      if (textAreaRef.current) {
        textAreaRef.current.focus()
        textAreaRef.current.setSelectionRange(nextCursor, nextCursor)
      }
    })
  }

  const ask = async (e) => {
    e.preventDefault()
    if (!activeThreadId) {
      setError('Create or choose a thread first.')
      return
    }
    const cleanMessage = stripDocMentions(message)
    if (!cleanMessage) {
      setError('Please type your question.')
      return
    }

    const forceDocIDs = [...mentionedDocIDs]
    const payload = {
      message: cleanMessage,
      scope_type: forceDocIDs.length > 0 ? 'doc' : 'auto',
      scope_doc_ids: forceDocIDs
    }

    setLoading(true)
    setError('')
    try {
      const draft = {
        turn: {
          id: `local_${Date.now()}`,
          question: cleanMessage,
          answer: '',
          scope_type: payload.scope_type
        },
        citations: [],
        retrieval_decision: null,
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
        } else if (event === 'retrieval_decision') {
          draft.retrieval_decision = payloadObj
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
          retrieval_decision: draft.retrieval_decision ? { ...draft.retrieval_decision } : null,
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
          retrieval_decision: draft.retrieval_decision ? { ...draft.retrieval_decision } : null,
          citations: [...draft.citations],
          items: []
        },
        ...prev
      ])
      setStreamingTurn(null)
      setMessage('')
      setMentionTarget(null)
      setMentionIndex(0)
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
        <p className="muted qa-hint">
          Type <code>@</code> to force this turn to use selected document RAG. Without <code>@</code>, retrieval mode is auto.
        </p>

        {mentionedDocs.length ? (
          <div className="mention-tags">
            {mentionedDocs.map((doc) => (
              <span key={doc.id} className="mention-tag">
                @{doc.name}
              </span>
            ))}
          </div>
        ) : null}

        <textarea
          ref={textAreaRef}
          rows={4}
          value={message}
          onChange={(e) => {
            const next = e.target.value
            setMessage(next)
            syncMentionTarget(next, e.target.selectionStart)
          }}
          onSelect={(e) => syncMentionTarget(e.target.value, e.target.selectionStart)}
          onKeyDown={(e) => {
            if (!mentionTarget || mentionSuggestions.length === 0) return
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              setMentionIndex((prev) => (prev + 1) % mentionSuggestions.length)
              return
            }
            if (e.key === 'ArrowUp') {
              e.preventDefault()
              setMentionIndex((prev) => (prev - 1 + mentionSuggestions.length) % mentionSuggestions.length)
              return
            }
            if (e.key === 'Enter') {
              e.preventDefault()
              applyMention(mentionSuggestions[mentionIndex] || mentionSuggestions[0])
              return
            }
            if (e.key === 'Escape') {
              e.preventDefault()
              setMentionTarget(null)
              setMentionIndex(0)
            }
          }}
          placeholder="Ask a grounded question..."
        />

        {mentionTarget && mentionSuggestions.length > 0 ? (
          <div className="mention-menu">
            {mentionSuggestions.map((doc, idx) => (
              <button
                key={doc.id}
                type="button"
                className={idx === mentionIndex ? 'mention-option active' : 'mention-option'}
                onClick={() => applyMention(doc)}
              >
                {doc.name}
              </button>
            ))}
          </div>
        ) : null}

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
            {streamingTurn.retrieval_decision ? (
              <p className="retrieval-decision muted">
                Retrieval: {streamingTurn.retrieval_decision.use_retrieval ? 'used' : 'skipped'} · {streamingTurn.retrieval_decision.reason || 'n/a'}
              </p>
            ) : null}
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
            {entry.retrieval_decision ? (
              <p className="retrieval-decision muted">
                Retrieval: {entry.retrieval_decision.use_retrieval ? 'used' : 'skipped'} · {entry.retrieval_decision.reason || 'n/a'}
              </p>
            ) : null}
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
