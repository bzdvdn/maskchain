import { useEffect, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
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
  Shield,
  Plus,
  Download,
  ChevronDown,
  FlaskConical,
} from 'lucide-react'
import { logout } from '../api/admin'
import { listTenants } from '../api/tenants'
import { useTheme } from '../hooks/useTheme'
import { useWorkspace } from '../hooks/useWorkspace'
import { CommandPalette } from './CommandPalette'

interface Props {
  children: React.ReactNode
  onLogout: () => void
}

interface NavItem {
  to: string
  label: string
  icon: typeof LayoutDashboard
}

type Section = { label: string; items: NavItem[] }

const navSections: Section[] = [
  {
    label: 'Overview',
    items: [
      { to: '/', label: 'Operations HQ', icon: LayoutDashboard },
      { to: '/analytics', label: 'Analytics', icon: ChartColumn },
    ],
  },
  {
    label: 'Traffic',
    items: [
      { to: '/sessions', label: 'Live Sessions', icon: Radio },
      { to: '/conversations', label: 'Conversations', icon: MessageSquare },
    ],
  },
  {
    label: 'Governance',
    items: [
      { to: '/tenants', label: 'Tenants', icon: Users },
      { to: '/keys', label: 'Keys', icon: KeyRound },
      { to: '/budgets', label: 'Budgets', icon: Wallet },
      { to: '/compliance', label: 'Compliance', icon: Shield },
    ],
  },
  {
    label: 'Operations',
    items: [
      { to: '/routing', label: 'Routing', icon: Route },
      { to: '/playground', label: 'Playground', icon: FlaskConical },
      { to: '/audit', label: 'Audit Log', icon: ScrollText },
    ],
  },
  {
    label: 'System',
    items: [
      { to: '/settings', label: 'Settings', icon: Settings },
      { to: '/swagger', label: 'API Reference', icon: FileJson },
    ],
  },
]

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
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()
  const [workspaces, setWorkspaces] = useState<{ slug: string; name: string }[]>([])
  const [workspace, applyWorkspace] = useWorkspace()

  useEffect(() => {
    const label = navSections.flatMap((s) => s.items).find((i) => i.to === location.pathname)?.label
    document.title = label ? `${label} — MaskChain` : 'MaskChain'
  }, [location.pathname])

  useEffect(() => {
    listTenants()
      .then((ts) => setWorkspaces(Array.isArray(ts) ? ts.map((t) => ({ slug: t.slug, name: t.name })) : []))
      .catch(() => {})
  }, [])

  function openCreateKey() {
    navigate('/keys?create=1')
  }

  function exportCSV() {
    window.open('/api/v1/analytics/cost?format=csv', '_blank')
  }

  const headerActions: { to: string; label: string; icon: typeof Plus; onClick?: () => void }[] = []
  if (location.pathname === '/') {
    headerActions.push({ to: '/keys', label: 'Create key', icon: Plus, onClick: openCreateKey })
  } else if (location.pathname === '/keys') {
    headerActions.push({ to: '/keys', label: 'Create key', icon: Plus, onClick: openCreateKey })
  } else if (location.pathname === '/analytics') {
    headerActions.push({ to: '', label: 'Export CSV', icon: Download, onClick: exportCSV })
  }

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
                  className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
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
          <h2>{navSections.flatMap((s) => s.items).find((i) => i.to === location.pathname)?.label ?? 'MaskChain'}</h2>
          <div className="header-right">
            {headerActions.map((a) => (
              <button key={a.label} type="button" className="header-action-btn btn btn-small" onClick={a.onClick}>
                <a.icon size={13} strokeWidth={2.2} />
                {a.label}
              </button>
            ))}
            <label className="workspace-switch">
              <span className="workspace-label">Workspace</span>
              <select value={workspace} onChange={(e) => applyWorkspace(e.target.value)} aria-label="Workspace scope">
                <option value="">All workspaces</option>
                {workspaces.map((w) => (
                  <option key={w.slug} value={w.slug}>{w.name}</option>
                ))}
              </select>
              <ChevronDown size={12} className="workspace-chevron" />
            </label>
            <span className="live-pill" title="Gateway status">
              <span className="live-dot" />
              Live
            </span>
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