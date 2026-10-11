import {
  Link,
  Navigate,
  NavLink,
  Route,
  Routes,
  useLocation,
} from 'react-router'
import EditPostingPage from './pages/EditPostingPage'
import NewPostingPage from './pages/NewPostingPage'
import PostingDetailPage from './pages/PostingDetailPage'
import PostingsPage from './pages/PostingsPage'
import StatusPage from './pages/StatusPage'

function navigationClass({ isActive }: { isActive: boolean }) {
  const base =
    'rounded-lg px-3 py-2 font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900'

  return `${base} ${isActive
    ? 'bg-slate-900 text-white'
    : 'text-slate-700 hover:bg-slate-100'
    }`
}

function App() {
  const { pathname } = useLocation()
  const isCapturePage = pathname === '/postings/new'

  return (
    <div className="min-h-screen bg-slate-50 font-sans text-slate-900">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 p-6">
          <p className="text-xl font-semibold">JST</p>

          <nav
            aria-label="Main navigation"
            className="flex flex-wrap gap-2"
          >
            <NavLink
              to="/postings"
              end={isCapturePage}
              className={navigationClass}
            >
              Postings
            </NavLink>

            <NavLink
              to="/postings/new"
              end
              className={navigationClass}
            >
              Add posting
            </NavLink>

            <NavLink
              to="/status"
              end
              className={navigationClass}
            >
              Status
            </NavLink>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-5xl p-6">
        <Routes>
          <Route
            path="/"
            element={<Navigate to="/postings" replace />}
          />
          <Route path="/postings" element={<PostingsPage />} />
          <Route path="/postings/new" element={<NewPostingPage />} />
          <Route
            path="/postings/:id"
            element={<PostingDetailPage />}
          />
          <Route
            path="/postings/:id/edit"
            element={<EditPostingPage />}
          />
          <Route path="/status" element={<StatusPage />} />
          <Route
            path="*"
            element={
              <section className="space-y-4">
                <h1 className="text-3xl font-semibold">
                  Page not found
                </h1>
                <p className="text-slate-600">
                  This page does not exist.
                </p>
                <Link
                  to="/postings"
                  className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
                >
                  Back to postings
                </Link>
              </section>
            }
          />
        </Routes>
      </main>
    </div>
  )
}

export default App
