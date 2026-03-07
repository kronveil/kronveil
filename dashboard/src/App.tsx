import { Routes, Route } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import Overview from './pages/Overview';
import Incidents from './pages/Incidents';
import Anomalies from './pages/Anomalies';
import Collectors from './pages/Collectors';
import Policies from './pages/Policies';
import Settings from './pages/Settings';

export default function App() {
  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-6">
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/incidents" element={<Incidents />} />
          <Route path="/anomalies" element={<Anomalies />} />
          <Route path="/collectors" element={<Collectors />} />
          <Route path="/policies" element={<Policies />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </main>
    </div>
  );
}
