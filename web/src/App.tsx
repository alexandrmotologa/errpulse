import { useState, useEffect, useCallback } from 'react';
import { Issue, IssueStatus, StatsSummary, Project, EventDetail } from './types';
import { useErrPulseStream } from './hooks/useErrPulseStream';
import { IssueTable } from './components/IssueTable';
import { IssueDetails } from './components/IssueDetails';
import { ReleaseManager } from './components/ReleaseManager';
import { AlertSettings } from './components/AlertSettings';
import { DsnHelper } from './components/DsnHelper';
import { EventSimulator } from './components/EventSimulator';
import {
  Activity,
  Flame,
  Package,
  Bell,
  Key,
  Radio,
  Sparkles,
  Layers,
  Database,
  Cpu,
} from 'lucide-react';

export function App() {
  const [activeTab, setActiveTab] = useState<'issues' | 'releases' | 'alerts' | 'dsn' | 'simulator'>('issues');
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<number>(1);
  const [stats, setStats] = useState<StatsSummary | null>(null);

  // Issues state
  const [issues, setIssues] = useState<Issue[]>([]);
  const [currentStatus, setCurrentStatus] = useState<IssueStatus | 'all'>('unresolved');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  const [latestEvent, setLatestEvent] = useState<EventDetail | null>(null);

  // Fetch projects
  const fetchProjects = async () => {
    try {
      const res = await fetch('/api/v1/projects');
      if (res.ok) {
        const data = await res.json();
        setProjects(data || []);
      }
    } catch {
      // Ignore
    }
  };

  // Fetch stats
  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch(`/api/v1/stats?project_id=${selectedProjectId}`);
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch {
      // Ignore
    }
  }, [selectedProjectId]);

  // Fetch issues
  const fetchIssues = useCallback(async () => {
    try {
      const params = new URLSearchParams({
        project_id: selectedProjectId.toString(),
        status: currentStatus,
        search: searchQuery,
      });
      const res = await fetch(`/api/v1/issues?${params.toString()}`);
      if (res.ok) {
        const data = await res.json();
        setIssues(data.issues || []);
      }
    } catch {
      // Ignore
    }
  }, [selectedProjectId, currentStatus, searchQuery]);

  // Select issue details
  const handleSelectIssue = async (issue: Issue) => {
    setSelectedIssue(issue);
    try {
      const res = await fetch(`/api/v1/issues/${issue.id}`);
      if (res.ok) {
        const data = await res.json();
        setLatestEvent(data.latest_event);
      }
    } catch {
      // Ignore
    }
  };

  // Handle live events via SSE
  const handleLiveEvent = useCallback(() => {
    fetchIssues();
    fetchStats();
  }, [fetchIssues, fetchStats]);

  const { connected } = useErrPulseStream(selectedProjectId, handleLiveEvent);

  useEffect(() => {
    fetchProjects();
  }, []);

  useEffect(() => {
    fetchStats();
    fetchIssues();
  }, [fetchStats, fetchIssues]);

  // Status transitions
  const handleQuickStatusChange = async (id: number, status: IssueStatus) => {
    try {
      const res = await fetch(`/api/v1/issues/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      });
      if (res.ok) {
        fetchIssues();
        fetchStats();
        if (selectedIssue && selectedIssue.id === id) {
          setSelectedIssue((prev) => (prev ? { ...prev, status } : null));
        }
      }
    } catch {
      // Ignore
    }
  };

  // Delete issue
  const handleDeleteIssue = async (id: number) => {
    try {
      const res = await fetch(`/api/v1/issues/${id}`, { method: 'DELETE' });
      if (res.ok) {
        setSelectedIssue(null);
        fetchIssues();
        fetchStats();
      }
    } catch {
      // Ignore
    }
  };

  const currentProject = projects.find((p) => p.id === selectedProjectId) || projects[0];

  return (
    <div className="min-h-screen flex flex-col bg-[#090a0f] text-slate-100">
      {/* Top Navbar */}
      <header className="border-b border-slate-800/80 bg-[#0d1017]/80 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center justify-between">
          <div className="flex items-center gap-6">
            {/* Logo */}
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shadow-sm shadow-emerald-500/20">
                <Activity className="w-5 h-5 animate-pulse" />
              </div>
              <span className="font-bold text-base tracking-tight text-white font-mono">
                Err<span className="text-emerald-400">Pulse</span>
              </span>
            </div>

            {/* Project Switcher */}
            {projects.length > 0 && (
              <div className="flex items-center gap-1.5 pl-3 border-l border-slate-800">
                <Layers className="w-3.5 h-3.5 text-slate-500" />
                <select
                  value={selectedProjectId}
                  onChange={(e) => setSelectedProjectId(Number(e.target.value))}
                  className="bg-transparent text-xs font-semibold text-slate-300 focus:outline-none cursor-pointer"
                >
                  {projects.map((p) => (
                    <option key={p.id} value={p.id} className="bg-[#121622] text-slate-200">
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>

          {/* Right controls */}
          <div className="flex items-center gap-4">
            {/* Live SSE Indicator */}
            <div
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-mono border transition-colors ${
                connected
                  ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30'
                  : 'bg-slate-800 text-slate-400 border-slate-700'
              }`}
            >
              <Radio className={`w-3 h-3 ${connected ? 'text-emerald-400 animate-pulse' : 'text-slate-500'}`} />
              <span>{connected ? 'LIVE' : 'CONNECTING...'}</span>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {/* Metric Summary Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="p-4 rounded-xl bg-[#0e111a] border border-slate-800/80">
            <span className="text-[11px] font-mono font-semibold uppercase text-slate-400 block mb-1">
              Total Ingested Events
            </span>
            <div className="text-2xl font-bold font-mono text-slate-100">
              {stats ? stats.total_events.toLocaleString() : '0'}
            </div>
          </div>

          <div className="p-4 rounded-xl bg-[#0e111a] border border-slate-800/80">
            <span className="text-[11px] font-mono font-semibold uppercase text-rose-400 block mb-1">
              Unresolved Issues
            </span>
            <div className="text-2xl font-bold font-mono text-rose-400">
              {stats ? stats.unresolved_issues.toLocaleString() : '0'}
            </div>
          </div>

          <div className="p-4 rounded-xl bg-[#0e111a] border border-slate-800/80">
            <span className="text-[11px] font-mono font-semibold uppercase text-emerald-400 block mb-1">
              Events Last 24h
            </span>
            <div className="text-2xl font-bold font-mono text-emerald-400">
              {stats ? stats.events_24h.toLocaleString() : '0'}
            </div>
          </div>

          <div className="p-4 rounded-xl bg-[#0e111a] border border-slate-800/80">
            <span className="text-[11px] font-mono font-semibold uppercase text-slate-400 block mb-1">
              Monitored Projects
            </span>
            <div className="text-2xl font-bold font-mono text-slate-100">
              {stats ? stats.total_projects : '1'}
            </div>
          </div>
        </div>

        {/* Studio Navigation Bar */}
        <div className="flex items-center gap-2 border-b border-slate-800 pb-2">
          <button
            onClick={() => setActiveTab('issues')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              activeTab === 'issues'
                ? 'bg-slate-800 text-white'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Flame className="w-3.5 h-3.5 text-rose-400" />
            <span>Issues</span>
            {stats && stats.unresolved_issues > 0 && (
              <span className="px-1.5 py-0.2 text-[10px] rounded-full bg-rose-500/20 text-rose-300 font-mono">
                {stats.unresolved_issues}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('releases')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              activeTab === 'releases'
                ? 'bg-slate-800 text-white'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Package className="w-3.5 h-3.5 text-sky-400" />
            <span>Releases & Maps</span>
          </button>

          <button
            onClick={() => setActiveTab('alerts')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              activeTab === 'alerts'
                ? 'bg-slate-800 text-white'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Bell className="w-3.5 h-3.5 text-amber-400" />
            <span>Alert Settings</span>
          </button>

          <button
            onClick={() => setActiveTab('dsn')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              activeTab === 'dsn'
                ? 'bg-slate-800 text-white'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Key className="w-3.5 h-3.5 text-emerald-400" />
            <span>SDK & DSN</span>
          </button>

          <button
            onClick={() => setActiveTab('simulator')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              activeTab === 'simulator'
                ? 'bg-slate-800 text-white'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
            <span>Simulator</span>
          </button>
        </div>

        {/* View Switcher */}
        {activeTab === 'issues' && (
          <IssueTable
            issues={issues}
            currentStatus={currentStatus}
            onStatusFilterChange={setCurrentStatus}
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            onSelectIssue={handleSelectIssue}
            onQuickStatusChange={handleQuickStatusChange}
            onDeleteIssue={handleDeleteIssue}
            unresolvedCount={stats?.unresolved_issues || 0}
          />
        )}

        {activeTab === 'releases' && (
          <ReleaseManager projectId={selectedProjectId} />
        )}

        {activeTab === 'alerts' && (
          <AlertSettings projectId={selectedProjectId} />
        )}

        {activeTab === 'dsn' && (
          <DsnHelper project={currentProject} />
        )}

        {activeTab === 'simulator' && (
          <EventSimulator onSimulated={handleLiveEvent} />
        )}
      </main>

      {/* Issue Details Modal Drawer */}
      {selectedIssue && (
        <IssueDetails
          issue={selectedIssue}
          latestEvent={latestEvent}
          onClose={() => setSelectedIssue(null)}
          onStatusChange={handleQuickStatusChange}
          onDelete={handleDeleteIssue}
        />
      )}

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-[#08090d] py-4 text-center text-xs text-slate-500 font-mono">
        <div className="flex items-center justify-center gap-4">
          <span className="flex items-center gap-1.5">
            <Database className="w-3.5 h-3.5 text-emerald-400" />
            SQLite WAL Mode
          </span>
          <span>•</span>
          <span className="flex items-center gap-1.5">
            <Cpu className="w-3.5 h-3.5 text-sky-400" />
            Memory Footprint &lt; 30MB RAM
          </span>
          <span>•</span>
          <span>Sentry Ingestion Protocol v7</span>
        </div>
      </footer>
    </div>
  );
}
