import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  AlertTriangle,
  Activity,
  Radio,
  Shield,
  BookOpen,
  Settings,
  Eye,
} from 'lucide-react';
import clsx from 'clsx';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Overview' },
  { to: '/incidents', icon: AlertTriangle, label: 'Incidents' },
  { to: '/anomalies', icon: Activity, label: 'Anomalies' },
  { to: '/collectors', icon: Radio, label: 'Collectors' },
  { to: '/policies', icon: Shield, label: 'Policies' },
  { to: '/runbooks', icon: BookOpen, label: 'Runbooks' },
  { to: '/settings', icon: Settings, label: 'Settings' },
];

export default function Sidebar() {
  return (
    <aside className="w-64 h-screen bg-[#070e1a] border-r border-[rgba(99,179,237,0.12)] flex flex-col">
      <div className="p-5 flex items-center gap-3 border-b border-[rgba(99,179,237,0.12)]">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-[#63b3ed] to-[#4fd1c5] flex items-center justify-center">
          <Eye className="w-4 h-4 text-[#040811]" />
        </div>
        <span className="font-display font-extrabold text-lg tracking-tight">
          Kronveil
        </span>
      </div>

      <nav className="flex-1 py-4 px-3">
        <ul className="space-y-1">
          {navItems.map(({ to, icon: Icon, label }) => (
            <li key={to}>
              <NavLink
                to={to}
                end={to === '/'}
                className={({ isActive }) =>
                  clsx(
                    'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors',
                    isActive
                      ? 'bg-[rgba(99,179,237,0.1)] text-[#63b3ed]'
                      : 'text-[#718096] hover:text-gray-200 hover:bg-[rgba(255,255,255,0.03)]',
                  )
                }
              >
                <Icon className="w-4 h-4" />
                {label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="p-4 border-t border-[rgba(99,179,237,0.12)]">
        <div className="flex items-center gap-2 text-xs text-[#718096]">
          <div className="w-2 h-2 rounded-full bg-[#68d391] animate-pulse" />
          Agent Connected
        </div>
        <div className="mt-1 font-mono text-[10px] text-[#4a5568]">
          v0.1.0 &middot; 3 clusters
        </div>
      </div>
    </aside>
  );
}
