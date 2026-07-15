import { Routes, Route, Navigate } from 'react-router-dom'
import { ProfileList, ProfileDetail, ProfileForm } from './pages/Profiles'
import { ErrorBoundary } from './components/ErrorBoundary'

// @sk-task 41-profiles-ui#T1.1: BrowserRouter + all route definitions (AC-008)
// @sk-task remove-audit-incidents#T3.6: Remove incident routes (AC-012, AC-013)
function App() {
  return (
    <div className="app">
      <ErrorBoundary>
        <header className="app-header">
          <a href="/profiles" className="app-logo">
            MaskChain
          </a>
          <nav>
            <a href="/profiles">Profiles</a>
          </nav>
        </header>
        <main className="app-main">
          <Routes>
            <Route path="/" element={<Navigate to="/profiles" replace />} />
            <Route path="/profiles" element={<ProfileList />} />
            <Route path="/profiles/new" element={<ProfileForm />} />
            <Route path="/profiles/:slug/edit" element={<ProfileForm />} />
            <Route path="/profiles/:slug" element={<ProfileDetail />} />
          </Routes>
        </main>
      </ErrorBoundary>
    </div>
  )
}

export default App
