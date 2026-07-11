import { Routes, Route, Navigate } from 'react-router-dom'
import { ProfileList, ProfileDetail, ProfileForm } from './pages/Profiles'
import { IncidentList, IncidentDetail } from './pages/Incidents'
import { ErrorBoundary } from './components/ErrorBoundary'

// @sk-task 41-profiles-ui#T1.1: BrowserRouter + all route definitions (AC-008)
// @sk-task 60-audit-incidents#T3.4: Add incident routes (AC-005)
function App() {
  return (
    <div className="app">
      <ErrorBoundary>
        <header className="app-header">
          <a href="/incidents" className="app-logo">
            MaskChain
          </a>
          <nav>
            <a href="/profiles">Profiles</a>
            <a href="/incidents">Incidents</a>
          </nav>
        </header>
        <main className="app-main">
          <Routes>
            <Route path="/" element={<Navigate to="/incidents" replace />} />
            <Route path="/profiles" element={<ProfileList />} />
            <Route path="/profiles/new" element={<ProfileForm />} />
            <Route path="/profiles/:slug/edit" element={<ProfileForm />} />
            <Route path="/profiles/:slug" element={<ProfileDetail />} />
            <Route path="/incidents" element={<IncidentList />} />
            <Route path="/incidents/:id" element={<IncidentDetail />} />
          </Routes>
        </main>
      </ErrorBoundary>
    </div>
  )
}

export default App
