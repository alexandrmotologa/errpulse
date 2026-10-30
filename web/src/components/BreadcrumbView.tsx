import React from 'react';
import { SentryBreadcrumb } from '../types';
import {
  MousePointer,
  Compass,
  Globe,
  Terminal,
  AlertTriangle,
  Info,
  Clock,
} from 'lucide-react';

interface BreadcrumbViewProps {
  breadcrumbs: SentryBreadcrumb[];
}

export const BreadcrumbView: React.FC<BreadcrumbViewProps> = ({ breadcrumbs = [] }) => {
  if (breadcrumbs.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-slate-500 italic">
        No breadcrumb trails recorded before this crash.
      </div>
    );
  }

  const getCategoryIcon = (category?: string, type?: string) => {
    const cat = (category || type || '').toLowerCase();
    if (cat.includes('click') || cat.includes('ui')) {
      return <MousePointer className="w-3.5 h-3.5 text-sky-400" />;
    }
    if (cat.includes('nav') || cat.includes('route')) {
      return <Compass className="w-3.5 h-3.5 text-indigo-400" />;
    }
    if (cat.includes('http') || cat.includes('fetch') || cat.includes('xhr')) {
      return <Globe className="w-3.5 h-3.5 text-emerald-400" />;
    }
    if (cat.includes('console') || cat.includes('log')) {
      return <Terminal className="w-3.5 h-3.5 text-amber-400" />;
    }
    if (cat.includes('error') || cat.includes('exception')) {
      return <AlertTriangle className="w-3.5 h-3.5 text-rose-400" />;
    }
    return <Info className="w-3.5 h-3.5 text-slate-400" />;
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 pb-2 border-b border-slate-800">
        <Clock className="w-4 h-4 text-emerald-400" />
        <span className="text-sm font-semibold text-slate-200">
          User Interaction Timeline ({breadcrumbs.length} steps)
        </span>
      </div>

      <div className="relative pl-6 space-y-4 before:absolute before:left-2.5 before:top-2 before:bottom-2 before:w-0.5 before:bg-slate-800">
        {breadcrumbs.map((b, idx) => (
          <div key={idx} className="relative group">
            {/* Timeline node */}
            <div className="absolute -left-[27px] top-1.5 w-5 h-5 rounded-full bg-[#121622] border border-slate-700 flex items-center justify-center shadow">
              {getCategoryIcon(b.category, b.type)}
            </div>

            <div className="p-3 rounded-lg bg-[#0e111a] border border-slate-800/80 hover:border-slate-700 transition-colors">
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2">
                  <span className="px-1.5 py-0.5 text-[10px] font-mono font-semibold uppercase rounded bg-slate-800 text-slate-300">
                    {b.category || b.type || 'generic'}
                  </span>
                  {b.level && b.level !== 'info' && (
                    <span
                      className={`text-[10px] font-bold uppercase ${
                        b.level === 'error'
                          ? 'text-rose-400'
                          : b.level === 'warn' || b.level === 'warning'
                          ? 'text-amber-400'
                          : 'text-slate-400'
                      }`}
                    >
                      [{b.level}]
                    </span>
                  )}
                </div>
              </div>

              <div className="font-mono text-xs text-slate-200 break-words">
                {b.message || (b.data ? JSON.stringify(b.data) : 'No message recorded')}
              </div>

              {b.data && Object.keys(b.data).length > 0 && b.message && (
                <pre className="mt-2 p-2 rounded bg-black/40 text-[11px] font-mono text-slate-400 overflow-x-auto">
                  {JSON.stringify(b.data, null, 2)}
                </pre>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
