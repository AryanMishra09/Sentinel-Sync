import { useState } from 'react'
import LandingPage from './pages/LandingPage'
import DashboardPage from './pages/DashboardPage'

export default function App() {
  const [page, setPage] = useState<'landing' | 'dashboard'>('landing')
  return page === 'landing'
    ? <LandingPage onEnter={() => setPage('dashboard')} />
    : <DashboardPage onBack={() => setPage('landing')} />
}
