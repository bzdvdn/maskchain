import { Routes, Route, Navigate } from 'react-router-dom'
import { TenantList, TenantDetail, TenantForm } from './pages/Tenants'
import { ErrorBoundary } from './components/ErrorBoundary'

function App() {
  return (
    <div className="app">
      <ErrorBoundary>
        <header className="app-header">
          <a href="/tenants" className="app-logo">
            MaskChain
          </a>
          <nav>
            <a href="/tenants">Tenants</a>
          </nav>
        </header>
        <main className="app-main">
          <Routes>
            <Route path="/" element={<Navigate to="/tenants" replace />} />
            <Route path="/tenants" element={<TenantList />} />
            <Route path="/tenants/new" element={<TenantForm />} />
            <Route path="/tenants/:slug/edit" element={<TenantForm />} />
            <Route path="/tenants/:slug" element={<TenantDetail />} />
          </Routes>
        </main>
      </ErrorBoundary>
    </div>
  )
}

export default App
