import { NavLink, useLocation } from 'react-router-dom'
import { logout } from '../api/admin'

interface Props {
  children: React.ReactNode
  onLogout: () => void
}

const navItems = [
  { to: '/', label: 'Dashboard', icon: '◉' },
  { to: '/analytics', label: 'Analytics', icon: '▦' },
  { to: '/tenants', label: 'Tenants', icon: '◆' },
  { to: '/sessions', label: 'Sessions', icon: '◎' },
  { to: '/routing', label: 'Routing', icon: '⇄' },
  { to: '/audit', label: 'Audit Log', icon: '☰' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
  { to: '/swagger', label: 'Swagger', icon: '⛁' },
]

const navSections: { label: string; items: typeof navItems }[] = [
  { label: 'Overview', items: navItems.slice(0, 2) },
  { label: 'Management', items: navItems.slice(2, 5) },
  { label: 'System', items: navItems.slice(5) },
]

const headerTimes: Record<string, string> = {
  '/': 'Last 30 min \u00b7 Auto-refresh 10s',
  '/analytics': 'Last 24h',
  '/tenants': '',
  '/sessions': 'Live',
  '/routing': 'Last check: 2s ago',
  '/audit': 'All time',
  '/settings': '',
  '/swagger': '',
}

export function Layout({ children, onLogout }: Props) {
  const location = useLocation()

  async function handleLogout() {
    await logout()
    onLogout()
  }

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-logo">⬡ MaskChain</div>
        <nav className="sidebar-nav">
          {navSections.map((section) => (
            <div key={section.label}>
              <div className="nav-section">{section.label}</div>
              {section.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/'}
                  className={({ isActive }) =>
                    `nav-item${isActive ? ' active' : ''}`
                  }
                >
                  <span className="nav-icon">{item.icon}</span>
                  <span>{item.label}</span>
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="sidebar-user">
            <div className="avatar">A</div>
            <span>admin</span>
          </div>
          <button type="button" className="btn-link" onClick={handleLogout}>
            Sign out
          </button>
        </div>
      </aside>
      <div className="main-area">
        <header className="app-header">
          <h2>
            {navItems.find((i) => i.to === location.pathname)?.label ?? 'MaskChain'}
          </h2>
          <div className="header-right">
            {headerTimes[location.pathname] && (
              <span className="time">{headerTimes[location.pathname]}</span>
            )}
          </div>
        </header>
        <main className="app-content">{children}</main>
      </div>
    </div>
  )
}
