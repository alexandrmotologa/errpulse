import React, { useState } from 'react';
import { Play, Sparkles, CheckCircle2, AlertCircle } from 'lucide-react';

interface EventSimulatorProps {
  onSimulated?: () => void;
}

export const EventSimulator: React.FC<EventSimulatorProps> = ({ onSimulated }) => {
  const [loadingScenario, setLoadingScenario] = useState<string | null>(null);
  const [result, setResult] = useState<{ text: string; error?: boolean } | null>(null);

  const simulate = async (scenario: string) => {
    setLoadingScenario(scenario);
    setResult(null);

    try {
      const res = await fetch('/api/v1/simulate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ scenario }),
      });

      const data = await res.json();
      if (res.ok) {
        setResult({ text: `Simulated crash "${scenario}" ingested (Event #${data.event_id})` });
        onSimulated?.();
      } else {
        setResult({ text: `Simulation error: ${data.error}`, error: true });
      }
    } catch (err: any) {
      setResult({ text: `Simulation failed: ${err.message}`, error: true });
    } finally {
      setLoadingScenario(null);
    }
  };

  return (
    <div className="p-6 rounded-xl bg-[#0e111a] border border-slate-800 space-y-4">
      <div>
        <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
          <Sparkles className="w-5 h-5 text-emerald-400" />
          Event Ingestion Simulator
        </h3>
        <p className="text-xs text-slate-400 mt-1">
          Generate realistic multi-language crash events directly into ErrPulse to evaluate stack traces, breadcrumb timelines, and alert webhooks.
        </p>
      </div>

      {result && (
        <div
          className={`p-3 rounded-lg text-xs font-medium flex items-center gap-2 ${
            result.error
              ? 'bg-rose-500/10 border border-rose-500/30 text-rose-300'
              : 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-300'
          }`}
        >
          {result.error ? (
            <AlertCircle className="w-4 h-4 shrink-0" />
          ) : (
            <CheckCircle2 className="w-4 h-4 shrink-0" />
          )}
          <span>{result.text}</span>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <button
          onClick={() => simulate('react_type_error')}
          disabled={!!loadingScenario}
          className="p-3 text-left rounded-lg bg-[#121622] hover:bg-slate-800 border border-slate-700/80 hover:border-emerald-500/40 transition-all flex flex-col justify-between group"
        >
          <div>
            <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-sky-400 block mb-1">
              Frontend Browser
            </span>
            <h4 className="text-xs font-bold text-slate-200 group-hover:text-emerald-300 transition-colors">
              React TypeError
            </h4>
            <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">
              Cannot read properties of undefined (reading 'calculateTotal') with user clicks and breadcrumbs.
            </p>
          </div>
          <div className="mt-3 flex items-center gap-1.5 text-xs font-medium text-emerald-400">
            <Play className="w-3.5 h-3.5" />
            <span>{loadingScenario === 'react_type_error' ? 'Injecting...' : 'Inject Error'}</span>
          </div>
        </button>

        <button
          onClick={() => simulate('go_panic')}
          disabled={!!loadingScenario}
          className="p-3 text-left rounded-lg bg-[#121622] hover:bg-slate-800 border border-slate-700/80 hover:border-emerald-500/40 transition-all flex flex-col justify-between group"
        >
          <div>
            <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-emerald-400 block mb-1">
              Backend Service
            </span>
            <h4 className="text-xs font-bold text-slate-200 group-hover:text-emerald-300 transition-colors">
              Go Runtime Panic
            </h4>
            <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">
              SIGSEGV nil pointer dereference during payment processing with context lines.
            </p>
          </div>
          <div className="mt-3 flex items-center gap-1.5 text-xs font-medium text-emerald-400">
            <Play className="w-3.5 h-3.5" />
            <span>{loadingScenario === 'go_panic' ? 'Injecting...' : 'Inject Error'}</span>
          </div>
        </button>

        <button
          onClick={() => simulate('python_key_error')}
          disabled={!!loadingScenario}
          className="p-3 text-left rounded-lg bg-[#121622] hover:bg-slate-800 border border-slate-700/80 hover:border-emerald-500/40 transition-all flex flex-col justify-between group"
        >
          <div>
            <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-amber-400 block mb-1">
              Worker Queue
            </span>
            <h4 className="text-xs font-bold text-slate-200 group-hover:text-emerald-300 transition-colors">
              Python Celery KeyError
            </h4>
            <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">
              Missing subscription_tier dictionary key in async background task execution.
            </p>
          </div>
          <div className="mt-3 flex items-center gap-1.5 text-xs font-medium text-emerald-400">
            <Play className="w-3.5 h-3.5" />
            <span>{loadingScenario === 'python_key_error' ? 'Injecting...' : 'Inject Error'}</span>
          </div>
        </button>
      </div>
    </div>
  );
};
