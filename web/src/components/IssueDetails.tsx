import React, { useState } from 'react';
import { Issue, EventDetail, IssueStatus } from '../types';
import { StackTraceViewer } from './StackTraceViewer';
import { BreadcrumbView } from './BreadcrumbView';
import {
  X,
  CheckCircle2,
  AlertCircle,
  Archive,
  Trash2,
  Calendar,
  Users,
  Hash,
  Tag,
  Code2,
  Copy,
  Check,
} from 'lucide-react';

interface IssueDetailsProps {
  issue: Issue;
  latestEvent?: EventDetail | null;
  onClose: () => void;
  onStatusChange: (id: number, status: IssueStatus) => void;
  onDelete: (id: number) => void;
}

export const IssueDetails: React.FC<IssueDetailsProps> = ({
  issue,
  latestEvent,
  onClose,
  onStatusChange,
  onDelete,
}) => {
  const [activeTab, setActiveTab] = useState<'stack' | 'breadcrumbs' | 'context' | 'raw'>('stack');
  const [copied, setCopied] = useState(false);

  // Parse JSON payloads safely
  let frames: any[] = [];
  try {
    if (latestEvent?.stacktrace_json) {
      const parsed = JSON.parse(latestEvent.stacktrace_json);
      if (Array.isArray(parsed) && parsed.length > 0 && parsed[0].stacktrace?.frames) {
        frames = parsed[0].stacktrace.frames;
      } else if (Array.isArray(parsed) && parsed.length > 0 && parsed[0].frames) {
        frames = parsed[0].frames;
      }
    }
  } catch {
    // Ignore json parse error
  }

  let breadcrumbs: any[] = [];
  try {
    if (latestEvent?.breadcrumbs_json) {
      breadcrumbs = JSON.parse(latestEvent.breadcrumbs_json);
    }
  } catch {
    // Ignore
  }

  let user: any = null;
  try {
    if (latestEvent?.user_json) {
      user = JSON.parse(latestEvent.user_json);
    }
  } catch {
    // Ignore
  }

  let tags: Record<string, string> = {};
  try {
    if (latestEvent?.tags_json) {
      tags = JSON.parse(latestEvent.tags_json);
    }
  } catch {
    // Ignore
  }

  let extra: Record<string, any> = {};
  try {
    if (latestEvent?.extra_json) {
      extra = JSON.parse(latestEvent.extra_json);
    }
  } catch {
    // Ignore
  }

  const copyRaw = () => {
    if (latestEvent?.raw_payload) {
      navigator.clipboard.writeText(latestEvent.raw_payload);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
      <div className="relative w-full max-w-5xl max-h-[92vh] flex flex-col rounded-xl bg-[#0d1017] border border-slate-800 shadow-2xl overflow-hidden">
        {/* Modal Header */}
        <div className="flex items-start justify-between p-6 border-b border-slate-800 bg-[#0a0c13]">
          <div className="space-y-2 pr-6">
            <div className="flex items-center gap-3">
              <span className="px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider rounded-md bg-rose-500/10 text-rose-400 border border-rose-500/20">
                {issue.type || 'Error'}
              </span>
              <span className="font-mono text-xs text-slate-500">
                ID: #{issue.id}
              </span>
              {issue.last_release && (
                <span className="px-2 py-0.5 text-xs font-mono rounded bg-slate-800 text-slate-300">
                  {issue.last_release}
                </span>
              )}
            </div>

            <h2 className="text-xl font-bold text-slate-100 break-words">
              {issue.title}
            </h2>

            <div className="font-mono text-xs text-emerald-400 flex items-center gap-1.5">
              <span>{issue.culprit}</span>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            {/* Status Selector */}
            <div className="flex items-center bg-[#131722] border border-slate-700/80 rounded-lg p-0.5">
              <button
                onClick={() => onStatusChange(issue.id, 'unresolved')}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors flex items-center gap-1.5 ${
                  issue.status === 'unresolved'
                    ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <AlertCircle className="w-3.5 h-3.5" />
                Unresolved
              </button>
              <button
                onClick={() => onStatusChange(issue.id, 'resolved')}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors flex items-center gap-1.5 ${
                  issue.status === 'resolved'
                    ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <CheckCircle2 className="w-3.5 h-3.5" />
                Resolved
              </button>
              <button
                onClick={() => onStatusChange(issue.id, 'ignored')}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors flex items-center gap-1.5 ${
                  issue.status === 'ignored'
                    ? 'bg-slate-700 text-slate-200'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Archive className="w-3.5 h-3.5" />
                Ignored
              </button>
            </div>

            <button
              onClick={() => onDelete(issue.id)}
              title="Delete issue"
              className="p-2 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
            >
              <Trash2 className="w-4 h-4" />
            </button>

            <button
              onClick={onClose}
              className="p-2 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Quick Stats Bar */}
        <div className="flex items-center gap-6 px-6 py-3 border-b border-slate-800/80 bg-[#0e111a] text-xs text-slate-400 font-mono">
          <div className="flex items-center gap-1.5">
            <Hash className="w-3.5 h-3.5 text-emerald-400" />
            <span>Events:</span>
            <strong className="text-slate-200">{issue.event_count}</strong>
          </div>
          <div className="flex items-center gap-1.5">
            <Users className="w-3.5 h-3.5 text-sky-400" />
            <span>Impacted Users:</span>
            <strong className="text-slate-200">{issue.user_count}</strong>
          </div>
          <div className="flex items-center gap-1.5">
            <Calendar className="w-3.5 h-3.5 text-slate-500" />
            <span>First seen:</span>
            <span>{new Date(issue.first_seen_at).toLocaleString()}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <Calendar className="w-3.5 h-3.5 text-slate-500" />
            <span>Last seen:</span>
            <span>{new Date(issue.last_seen_at).toLocaleString()}</span>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="flex items-center gap-1 px-6 pt-3 border-b border-slate-800 bg-[#0b0e14]">
          <button
            onClick={() => setActiveTab('stack')}
            className={`px-4 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'stack'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Stack Trace
          </button>
          <button
            onClick={() => setActiveTab('breadcrumbs')}
            className={`px-4 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'breadcrumbs'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Breadcrumbs ({breadcrumbs.length})
          </button>
          <button
            onClick={() => setActiveTab('context')}
            className={`px-4 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'context'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Tags & Context
          </button>
          <button
            onClick={() => setActiveTab('raw')}
            className={`px-4 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'raw'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Raw JSON
          </button>
        </div>

        {/* Tab Content Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {activeTab === 'stack' && (
            frames.length > 0 ? (
              <StackTraceViewer frames={frames} />
            ) : (
              <div className="py-12 text-center text-sm text-slate-500 italic">
                No stack trace available for this crash event.
              </div>
            )
          )}

          {activeTab === 'breadcrumbs' && (
            <BreadcrumbView breadcrumbs={breadcrumbs} />
          )}

          {activeTab === 'context' && (
            <div className="space-y-6">
              {/* User Telemetry */}
              {user && (
                <div className="p-4 rounded-lg bg-[#0e111a] border border-slate-800">
                  <h4 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3 flex items-center gap-1.5">
                    <Users className="w-3.5 h-3.5 text-sky-400" />
                    Impacted User
                  </h4>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs font-mono">
                    <div>
                      <span className="text-slate-500 block">ID</span>
                      <span className="text-slate-200">{user.id || 'anonymous'}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block">Email</span>
                      <span className="text-slate-200">{user.email || 'none'}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block">IP Address</span>
                      <span className="text-slate-200">{user.ip_address || 'unknown'}</span>
                    </div>
                  </div>
                </div>
              )}

              {/* Tags Cloud */}
              {Object.keys(tags).length > 0 && (
                <div className="p-4 rounded-lg bg-[#0e111a] border border-slate-800">
                  <h4 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3 flex items-center gap-1.5">
                    <Tag className="w-3.5 h-3.5 text-emerald-400" />
                    Tags
                  </h4>
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(tags).map(([k, v]) => (
                      <div
                        key={k}
                        className="flex items-center text-xs font-mono rounded bg-slate-800/80 border border-slate-700/60 overflow-hidden"
                      >
                        <span className="px-2 py-1 bg-slate-800 text-slate-400 border-r border-slate-700/60">
                          {k}
                        </span>
                        <span className="px-2 py-1 text-slate-200 font-semibold">{v}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Extra context */}
              {Object.keys(extra).length > 0 && (
                <div className="p-4 rounded-lg bg-[#0e111a] border border-slate-800">
                  <h4 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3 flex items-center gap-1.5">
                    <Code2 className="w-3.5 h-3.5 text-indigo-400" />
                    Extra Context
                  </h4>
                  <pre className="p-3 rounded bg-black/50 text-xs font-mono text-slate-300 overflow-x-auto">
                    {JSON.stringify(extra, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}

          {activeTab === 'raw' && (
            <div className="relative">
              <button
                onClick={copyRaw}
                className="absolute right-3 top-3 flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors"
              >
                {copied ? (
                  <>
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3.5 h-3.5 text-slate-400" />
                    <span>Copy JSON</span>
                  </>
                )}
              </button>
              <pre className="p-4 rounded-lg bg-black/60 border border-slate-800 text-xs font-mono text-slate-300 overflow-x-auto max-h-[60vh]">
                {latestEvent?.raw_payload || JSON.stringify(latestEvent, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
