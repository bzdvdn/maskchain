import { useState, useCallback, useEffect } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { TenantList, TenantDetail, TenantForm } from './pages/Tenants'
import { Dashboard } from './pages/Dashboard'
import { Analytics } from './pages/Analytics'
import { Sessions } from './pages/Sessions'
import { Routing } from './pages/Routing'
import { AuditLog } from './pages/AuditLog'
import { Settings } from './pages/Settings'
import { Swagger } from './pages/Swagger'
import { Login } from './pages/Login'
import { Layout } from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import { getAdminToken, setAdminToken } from './api/admin'

function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(() => !!getAdminToken())
  const [checking, setChecking] = useState(() => !!getAdminToken())

  useEffect(() => {
    const token = getAdminToken()
    if (!token) {
      setChecking(false)
      return
    }
    fetch('/api/v1/admin/verify', {
      headers: { 'Authorization': `Bearer ${token}` },
      credentials: 'include',
    }).then((r) => {
      if (!r.ok) {
        setAdminToken(null)
        setIsLoggedIn(false)
      }
    }).catch(() => {
      setAdminToken(null)
      setIsLoggedIn(false)
    }).finally(() => setChecking(false))
  }, [])

  const handleLogin = useCallback(() => {
    setIsLoggedIn(true)
  }, [])

  const handleLogout = useCallback(() => {
    setIsLoggedIn(false)
  }, [])

  if (checking) return null
  if (!isLoggedIn) {
    return <Login onLogin={handleLogin} />
  }

  return (
    <ErrorBoundary>
      <Layout onLogout={handleLogout}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/tenants" element={<TenantList />} />
          <Route path="/tenants/new" element={<TenantForm />} />
          <Route path="/tenants/:slug/edit" element={<TenantForm />} />
          <Route path="/tenants/:slug" element={<TenantDetail />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="/sessions" element={<Sessions />} />
          <Route path="/routing" element={<Routing />} />
          <Route path="/audit" element={<AuditLog />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/swagger" element={<Swagger />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </ErrorBoundary>
  )
}

export default App
