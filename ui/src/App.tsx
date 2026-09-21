import { useState, useCallback, useEffect, lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { Login } from './pages/Login'
import { Layout } from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Spinner } from './components/ui'
import { getAdminToken, setAdminToken } from './api/admin'

// @sk-task conversation-logging#T3.2: Add /conversations route (AC-005, AC-006)
const TenantList = lazy(() => import('./pages/Tenants/TenantList').then((m) => ({ default: m.TenantList })))
const TenantDetail = lazy(() => import('./pages/Tenants/TenantDetail').then((m) => ({ default: m.TenantDetail })))
const TenantForm = lazy(() => import('./pages/Tenants/TenantForm').then((m) => ({ default: m.TenantForm })))
const Dashboard = lazy(() => import('./pages/Dashboard').then((m) => ({ default: m.Dashboard })))
const Analytics = lazy(() => import('./pages/Analytics').then((m) => ({ default: m.Analytics })))
const Sessions = lazy(() => import('./pages/Sessions').then((m) => ({ default: m.Sessions })))
const Conversations = lazy(() => import('./pages/Conversations').then((m) => ({ default: m.Conversations })))
const Routing = lazy(() => import('./pages/Routing').then((m) => ({ default: m.Routing })))
const Providers = lazy(() => import('./pages/Providers').then((m) => ({ default: m.Providers })))
const Models = lazy(() => import('./pages/Models').then((m) => ({ default: m.Models })))
const Keys = lazy(() => import('./pages/Keys').then((m) => ({ default: m.Keys })))
const Budgets = lazy(() => import('./pages/Budgets').then((m) => ({ default: m.Budgets })))
const AuditLog = lazy(() => import('./pages/AuditLog').then((m) => ({ default: m.AuditLog })))
const Settings = lazy(() => import('./pages/Settings').then((m) => ({ default: m.Settings })))
const Swagger = lazy(() => import('./pages/Swagger').then((m) => ({ default: m.Swagger })))
const Compliance = lazy(() => import('./pages/Compliance').then((m) => ({ default: m.Compliance })))
const Playground = lazy(() => import('./pages/Playground').then((m) => ({ default: m.Playground })))

function PageBoundary({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return (
    <ErrorBoundary resetKey={location.pathname}>
      <Suspense fallback={<Spinner label="Loading..." />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

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

  useEffect(() => {
    const onUnauthorized = () => {
      setAdminToken(null)
      setIsLoggedIn(false)
    }
    window.addEventListener('maskchain:unauthorized', onUnauthorized)
    return () => window.removeEventListener('maskchain:unauthorized', onUnauthorized)
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
          <Route path="/" element={<PageBoundary><Dashboard /></PageBoundary>} />
          <Route path="/tenants" element={<PageBoundary><TenantList /></PageBoundary>} />
          <Route path="/tenants/new" element={<PageBoundary><TenantForm /></PageBoundary>} />
          <Route path="/tenants/:slug/edit" element={<PageBoundary><TenantForm /></PageBoundary>} />
          <Route path="/tenants/:slug" element={<PageBoundary><TenantDetail /></PageBoundary>} />
          <Route path="/analytics" element={<PageBoundary><Analytics /></PageBoundary>} />
          <Route path="/sessions" element={<PageBoundary><Sessions /></PageBoundary>} />
          <Route path="/conversations" element={<PageBoundary><Conversations /></PageBoundary>} />
          <Route path="/providers" element={<PageBoundary><Providers /></PageBoundary>} />
          <Route path="/models" element={<PageBoundary><Models /></PageBoundary>} />
          <Route path="/routing" element={<PageBoundary><Routing /></PageBoundary>} />
          <Route path="/keys" element={<PageBoundary><Keys /></PageBoundary>} />
          <Route path="/budgets" element={<PageBoundary><Budgets /></PageBoundary>} />
          <Route path="/compliance" element={<PageBoundary><Compliance /></PageBoundary>} />
          <Route path="/playground" element={<PageBoundary><Playground /></PageBoundary>} />
          <Route path="/audit" element={<PageBoundary><AuditLog /></PageBoundary>} />
          <Route path="/settings" element={<PageBoundary><Settings /></PageBoundary>} />
          <Route path="/swagger" element={<PageBoundary><Swagger /></PageBoundary>} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </ErrorBoundary>
  )
}

export default App