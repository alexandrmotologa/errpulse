import React, { useState } from 'react';
import { Issue, IssueStatus } from '../types';
import { Sparkline } from './Sparkline';
import {
  Search,
  CheckCircle2,
  AlertCircle,
  Archive,
  Trash2,
  Users,
  Flame,
  Clock,
  ArrowUpDown,
} from 'lucide-react';

interface IssueTableProps {
  issues: Issue[];
  currentStatus: IssueStatus | 'all';
  onStatusFilterChange: (status: IssueStatus | 'all') => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onSelectIssue: (issue: Issue) => void;
  onQuickStatusChange: (id: number, status: IssueStatus) => void;
  onDeleteIssue: (id: number) => void;
  unresolvedCount: number;
}

export const IssueTable: React.FC<IssueTableProps> = ({
  issues,
  currentStatus,
  onStatusFilterChange,
  searchQuery,
  onSearchChange,
  onSelectIssue,
  onQuickStatusChange,
  onDeleteIssue,
  unresolvedCount,
}) => {
  const [selectedIDs, setSelectedIDs] = useState<Record<number, boolean>>({});

  const toggleSelectAll = () => {
    if (Object.keys(selectedIDs).length === issues.length && issues.length > 0) {
      setSelectedIDs({});
    } else {
      const all: Record<number, boolean> = {};
      issues.forEach((i) => {
        all[i.id] = true;
      });
      setSelectedIDs(all);
    }
  };

  const toggleSelectOne = (id: number, e: React.MouseEvent) => {
    e.stopPropagation();
    setSelectedIDs((prev) => {
      const next = { ...prev };
      if (next[id]) delete next[id];
      else next[id] = true;
      return next;
    });
  };

  const bulkResolve = () => {
    Object.keys(selectedIDs).forEach((idStr) => {
      onQuickStatusChange(Number(idStr), 'resolved');
    });
    setSelectedIDs({});
  };

  const selectedCount = Object.keys(selectedIDs).length;

  return (
    <div className="space-y-4">
      {/* Controls Bar */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-4">
        {/* Status Filter Tabs */}
        <div className="flex items-center p-1 bg-[#121622] border border-slate-800 rounded-lg">
          <button
            onClick={() => onStatusFilterChange('unresolved')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${
              currentStatus === 'unresolved'
                ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <AlertCircle className="w-3.5 h-3.5" />
            Unresolved
            {unresolvedCount > 0 && (
              <span className="px-1.5 py-0.2 text-[10px] font-bold rounded-full bg-rose-500/30 text-rose-200">
                {unresolvedCount}
              </span>
            )}
          </button>
          <button
            onClick={() => onStatusFilterChange('resolved')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${
              currentStatus === 'resolved'
                ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <CheckCircle2 className="w-3.5 h-3.5" />
            Resolved
          </button>
          <button
            onClick={() => onStatusFilterChange('ignored')}
            className={`flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${
              currentStatus === 'ignored'
                ? 'bg-slate-700 text-slate-200'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Archive className="w-3.5 h-3.5" />
            Ignored
          </button>
          <button
            onClick={() => onStatusFilterChange('all')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-md transition-colors ${
              currentStatus === 'all'
                ? 'bg-slate-700 text-slate-200'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            All
          </button>
        </div>

        {/* Search Input */}
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-2.5 w-4 h-4 text-slate-500" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search issues, culprits, or types..."
            className="w-full pl-9 pr-4 py-1.5 text-xs bg-[#10131d] border border-slate-800 rounded-lg text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500 transition-colors"
          />
        </div>
      </div>

      {/* Bulk actions banner if items selected */}
      {selectedCount > 0 && (
        <div className="flex items-center justify-between px-4 py-2 bg-emerald-500/10 border border-emerald-500/30 rounded-lg text-xs text-emerald-300">
          <span>{selectedCount} issues selected</span>
          <div className="flex items-center gap-2">
            <button
              onClick={bulkResolve}
              className="flex items-center gap-1.5 px-3 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded font-medium transition-colors"
            >
              <CheckCircle2 className="w-3.5 h-3.5" />
              Resolve Selected
            </button>
          </div>
        </div>
      )}

      {/* Issues Table */}
      <div className="rounded-xl border border-slate-800/80 bg-[#0e111a] overflow-hidden shadow">
        <div className="grid grid-cols-12 gap-4 px-4 py-3 border-b border-slate-800 bg-[#0a0c13] text-[11px] font-bold uppercase tracking-wider text-slate-400">
          <div className="col-span-6 flex items-center gap-3">
            <input
              type="checkbox"
              checked={issues.length > 0 && selectedCount === issues.length}
              onChange={toggleSelectAll}
              className="rounded bg-slate-900 border-slate-700 text-emerald-500 focus:ring-0 cursor-pointer"
            />
            <span>Issue & Culprit</span>
          </div>
          <div className="col-span-2 flex items-center gap-1">
            <ArrowUpDown className="w-3 h-3 text-slate-500" />
            <span>Activity (24h)</span>
          </div>
          <div className="col-span-1 text-right">Events</div>
          <div className="col-span-1 text-right">Users</div>
          <div className="col-span-2 text-right">Last Seen</div>
        </div>

        {issues.length === 0 ? (
          <div className="py-16 text-center text-slate-500 space-y-2">
            <Flame className="w-8 h-8 mx-auto text-slate-600 opacity-60" />
            <p className="text-sm font-medium">No crash issues matching your filters.</p>
            <p className="text-xs text-slate-600">
              New crashes ingested via Sentry envelope protocol will appear here automatically.
            </p>
          </div>
        ) : (
          <div className="divide-y divide-slate-800/60">
            {issues.map((issue) => {
              const isSelected = !!selectedIDs[issue.id];
              const lastSeenDate = new Date(issue.last_seen_at);

              return (
                <div
                  key={issue.id}
                  onClick={() => onSelectIssue(issue)}
                  className={`grid grid-cols-12 gap-4 px-4 py-3.5 items-center cursor-pointer hover:bg-slate-800/30 transition-colors group ${
                    isSelected ? 'bg-emerald-500/5' : ''
                  }`}
                >
                  {/* Issue & Culprit */}
                  <div className="col-span-6 flex items-start gap-3 overflow-hidden">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onClick={(e) => toggleSelectOne(issue.id, e)}
                      onChange={() => {}}
                      className="mt-1 rounded bg-slate-900 border-slate-700 text-emerald-500 focus:ring-0 cursor-pointer"
                    />

                    <div className="min-w-0 space-y-1">
                      <div className="flex items-center gap-2">
                        <span
                          className={`font-semibold text-xs truncate group-hover:text-emerald-300 transition-colors ${
                            issue.status === 'resolved'
                              ? 'line-through text-slate-400'
                              : 'text-slate-100'
                          }`}
                        >
                          {issue.title}
                        </span>
                        {issue.last_release && (
                          <span className="shrink-0 px-1.5 py-0.2 text-[10px] font-mono rounded bg-slate-800 text-slate-400">
                            {issue.last_release}
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-2 text-xs font-mono text-slate-500 truncate">
                        <span className="text-emerald-400 truncate">{issue.culprit}</span>
                      </div>
                    </div>
                  </div>

                  {/* 24h Sparkline */}
                  <div className="col-span-2">
                    <Sparkline data={issue.sparkline} />
                  </div>

                  {/* Events */}
                  <div className="col-span-1 text-right font-mono text-xs font-semibold text-slate-200">
                    {issue.event_count.toLocaleString()}
                  </div>

                  {/* Impacted Users */}
                  <div className="col-span-1 text-right font-mono text-xs text-slate-300 flex items-center justify-end gap-1">
                    <Users className="w-3 h-3 text-sky-400 opacity-60" />
                    {issue.user_count.toLocaleString()}
                  </div>

                  {/* Last Seen / Quick Actions on hover */}
                  <div className="col-span-2 text-right relative flex items-center justify-end">
                    <div className="text-xs text-slate-400 flex items-center gap-1 group-hover:hidden">
                      <Clock className="w-3 h-3 text-slate-500" />
                      <span>{lastSeenDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
                    </div>

                    {/* Hover Quick Action Buttons */}
                    <div
                      onClick={(e) => e.stopPropagation()}
                      className="hidden group-hover:flex items-center gap-1"
                    >
                      {issue.status !== 'resolved' ? (
                        <button
                          onClick={() => onQuickStatusChange(issue.id, 'resolved')}
                          title="Mark Resolved"
                          className="p-1 rounded bg-slate-800 hover:bg-emerald-500/20 hover:text-emerald-400 text-slate-400 transition-colors"
                        >
                          <CheckCircle2 className="w-4 h-4" />
                        </button>
                      ) : (
                        <button
                          onClick={() => onQuickStatusChange(issue.id, 'unresolved')}
                          title="Reopen Issue"
                          className="p-1 rounded bg-slate-800 hover:bg-rose-500/20 hover:text-rose-400 text-slate-400 transition-colors"
                        >
                          <AlertCircle className="w-4 h-4" />
                        </button>
                      )}

                      <button
                        onClick={() => onDeleteIssue(issue.id)}
                        title="Delete Issue"
                        className="p-1 rounded bg-slate-800 hover:bg-rose-500/20 hover:text-rose-400 text-slate-400 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
