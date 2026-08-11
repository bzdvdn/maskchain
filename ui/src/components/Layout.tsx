import { useEffect } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  ChartColumn,
  Users,
  Radio,
  MessageSquare,
  Route,
  ScrollText,
  Settings,
  FileJson,
  LogOut,
  Hexagon,
  Moon,
  Sun,
  Search,
  KeyRound,
  Wallet,
} from 'lucide-react'
import { logout } from '../api/admin'
import { useTheme } from '../hooks/useTheme'
import { CommandPalette } from './CommandPalette'

interface Props {
  children: React.ReactNode
  onLogout: () => void
}

// @sk-task conversation-logging#T3.2: Add Conversations menu item in Management section (AC-005, AC-006)
const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/analytics', label: 'Analytics', icon: ChartColumn },
  { to: '/tenants', label: 'Tenants', icon: Users },
  { to: '/sessions', label: 'Sessions', icon: Radio },
  { to: '/conversations', label: 'Conversations', icon: MessageSquare },
  { to: '/routing', label: 'Routing', icon: Route },
  { to: '/keys', label: 'Keys', icon: KeyRound },
  { to: '/budgets', label: 'Budgets', icon: Wallet },
  { to: '/audit', label: 'Audit Log', icon: ScrollText },
  { to: '/settings', label: 'Settings', icon: Settings },
  { to: '/swagger', label: 'Swagger', icon: FileJson },
]

const navSections: { label: string; items: typeof navItems }[] = [
  { label: 'Overview', items: navItems.slice(0, 2) },
  { label: 'Management', items: navItems.slice(2, 8) },
  { label: 'System', items: navItems.slice(8) },
]

const headerTimes: Record<string, string> = {
  '/sessions': 'Live',
  '/routing': 'Last check: 2s ago',
  '/audit': 'All time',
}

function Logo() {
  return (
    <div className="sidebar-logo">
      <Hexagon size={20} strokeWidth={2.2} />
      <span>MaskChain</span>
    </div>
  )
}

export function Layout({ children, onLogout }: Props) {
  const location = useLocation()
  const { theme, toggleTheme } = useTheme()

  useEffect(() => {
    const label = navItems.find((i) => i.to === location.pathname)?.label
    document.title = label ? `${label} — MaskChain` : 'MaskChain'
  }, [location.pathname])

  async function handleLogout() {
    await logout()
    onLogout()
  }

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <Logo />
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
                  <item.icon size={16} strokeWidth={2} className="nav-icon" />
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
          <button type="button" className="btn-link btn-link-logout" onClick={handleLogout}>
            <LogOut size={12} />
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
            <button
              type="button"
              className="btn-link command-trigger"
              onClick={() => window.dispatchEvent(new Event('maskchain:command-open'))}
              aria-label="Open search (Ctrl+K)"
              title="Search (Ctrl+K)"
            >
              <Search size={14} />
            </button>
            <button
              type="button"
              className="btn-link"
              onClick={toggleTheme}
              aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
              title={theme === 'dark' ? 'Light theme' : 'Dark theme'}
            >
              {theme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
            </button>
          </div>
        </header>
        <main className="app-content">{children}</main>
      </div>
      <CommandPalette />
    </div>
  )
}
