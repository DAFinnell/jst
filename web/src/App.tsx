import { useEffect, useState } from 'react'
import { getHealth } from './api/health'

type ConnectionStatus = 'loading' | 'connected' | 'error'

function App() {
  const [status, setStatus] = useState<ConnectionStatus>('loading')
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController

    void getHealth(controller.signal)
      .then(() => {
        if (!controller.signal.aborted) {
          setStatus('connected')
        }
      })
        .catch(() => {
        if (!controller.signal.aborted) {
          setStatus('error')
        }
      })

    return () => controller.abort()
  }, [attempt])

  function retry() {
    setStatus('loading')
    setAttempt((current) => current + 1)
  }

  const message = {
    loading: 'Checking connection...',
    connected: 'Backend connected.',
    error: 'Unable to connect. Try again.',
  }[status]

  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 p-6 font-sans text-slate-900">
      <section className="w-full max-w-md rounded-xl border border-slate-200 bg-white p-8 shadow-sm">
        <h1 className="text-3xl font-semibold">JST</h1>
        <p className="mt-2 text-slate-600">Job Search Tracker</p>

        <p
          aria-live="polite"
          className={`mt-8 font-medium ${
            status === 'error' ? 'text-red-700' : 'text-slate-700'
          }`}
        >
          {message}
        </p>

        <button
          type="button"
          onClick={retry}
          disabled={status === 'loading'}
          className="mt-4 rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:cursor-wait disabled:opacity-50"
        >
          {status === 'loading'
            ? 'Checking...'
            : status === 'error'
              ? 'Retry'
              : 'Check again'}
        </button>
      </section>
    </main>
  )
}

export default App
