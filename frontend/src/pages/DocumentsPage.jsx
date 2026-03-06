import { useEffect, useState } from 'react'
import { api } from '../api'

export function DocumentsPage({ token, user }) {
  const [documents, setDocuments] = useState([])
  const [selectedFile, setSelectedFile] = useState(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const loadDocuments = async () => {
    try {
      const res = await api.listDocuments(token)
      setDocuments(res.documents || [])
      setError('')
    } catch (err) {
      setError(err.message)
    }
  }

  useEffect(() => {
    loadDocuments()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  const upload = async (e) => {
    e.preventDefault()
    if (!selectedFile) {
      setError('Please choose a file first.')
      return
    }

    setLoading(true)
    setError('')
    setMessage('Uploading and indexing...')
    try {
      await api.uploadDocument(token, selectedFile)
      setSelectedFile(null)
      setMessage('Upload complete and document indexed.')
      await loadDocuments()
    } catch (err) {
      setError(err.message)
      setMessage('')
    } finally {
      setLoading(false)
    }
  }

  const removeDoc = async (doc) => {
    const ok = window.confirm(`Delete ${doc.name}? This action removes it from retrieval index.`)
    if (!ok) {
      return
    }
    try {
      await api.deleteDocument(token, doc.id)
      await loadDocuments()
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <section className="panel">
      <h2>Document Management</h2>
      <p className="muted">Current user: <strong>{user.email}</strong></p>

      <form className="upload-form" onSubmit={upload}>
        <input
          type="file"
          accept=".txt,.md,.markdown,.pdf"
          onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
        />
        <button type="submit" disabled={loading}>{loading ? 'Uploading...' : 'Upload'}</button>
      </form>

      {message ? <div className="message ok">{message}</div> : null}
      {error ? <div className="message error">{error}</div> : null}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Chunks</th>
              <th>Created</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {documents.length === 0 ? (
              <tr>
                <td colSpan={5} className="empty">No documents yet.</td>
              </tr>
            ) : (
              documents.map((doc) => (
                <tr key={doc.id}>
                  <td>{doc.name}</td>
                  <td><span className={`status status-${doc.status}`}>{doc.status}</span></td>
                  <td>{doc.chunk_count}</td>
                  <td>{new Date(doc.created_at).toLocaleString()}</td>
                  <td>
                    <button className="danger-btn" onClick={() => removeDoc(doc)}>Delete</button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </section>
  )
}
