import React, { useState } from 'react';
import { SentryFrame } from '../types';
import { FileCode, Eye, ChevronDown, ChevronRight, Layers } from 'lucide-react';

interface StackTraceViewerProps {
  frames: SentryFrame[];
}

export const StackTraceViewer: React.FC<StackTraceViewerProps> = ({ frames = [] }) => {
  const [onlyInApp, setOnlyInApp] = useState(true);
  const [expandedIndices, setExpandedIndices] = useState<Record<number, boolean>>({});

  const displayFrames = onlyInApp
    ? frames.filter((f) => f.in_app !== false)
    : frames;

  // Toggle frame expansion
  const toggleFrame = (idx: number) => {
    setExpandedIndices((prev) => ({
      ...prev,
      [idx]: !prev[idx],
    }));
  };

  const hasInApp = frames.some((f) => f.in_app !== false);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between pb-2 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <Layers className="w-4 h-4 text-emerald-400" />
          <span className="text-sm font-semibold text-slate-200">
            Stack Trace ({displayFrames.length} frames)
          </span>
        </div>
        {hasInApp && (
          <button
            onClick={() => setOnlyInApp(!onlyInApp)}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
          >
            <Eye className="w-3.5 h-3.5 text-emerald-400" />
            {onlyInApp ? 'Showing App Only' : 'Showing All Frames'}
          </button>
        )}
      </div>

      <div className="space-y-2">
        {displayFrames.map((frame, idx) => {
          const isExpanded = expandedIndices[idx] !== undefined ? expandedIndices[idx] : (frame.context_line !== undefined || idx === displayFrames.length - 1);
          const hasContext = !!frame.context_line;

          return (
            <div
              key={idx}
              className={`rounded-lg border transition-all ${
                frame.in_app !== false
                  ? 'border-emerald-500/30 bg-[#0d111a]'
                  : 'border-slate-800 bg-[#0a0c12] opacity-75'
              }`}
            >
              {/* Frame Header */}
              <div
                onClick={() => hasContext && toggleFrame(idx)}
                className={`flex items-center justify-between px-3.5 py-2.5 select-none ${
                  hasContext ? 'cursor-pointer hover:bg-slate-800/40' : ''
                }`}
              >
                <div className="flex items-center gap-2 overflow-hidden">
                  {hasContext ? (
                    isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-slate-400 shrink-0" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-slate-400 shrink-0" />
                    )
                  ) : (
                    <FileCode className="w-4 h-4 text-slate-500 shrink-0" />
                  )}

                  <div className="truncate">
                    <span className="font-mono text-xs font-semibold text-emerald-400">
                      {frame.function || '<anonymous>'}
                    </span>
                    <span className="mx-2 text-slate-600 font-mono text-xs">in</span>
                    <span className="font-mono text-xs text-slate-300">
                      {frame.filename}:{frame.lineno}
                      {frame.colno ? `:${frame.colno}` : ''}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  {frame.in_app !== false ? (
                    <span className="px-1.5 py-0.5 text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 rounded border border-emerald-500/20">
                      In-App
                    </span>
                  ) : (
                    <span className="px-1.5 py-0.5 text-[10px] font-medium bg-slate-800 text-slate-400 rounded">
                      Vendor
                    </span>
                  )}
                </div>
              </div>

              {/* Code Context Lines */}
              {isExpanded && hasContext && (
                <div className="px-3 pb-3 pt-1 border-t border-slate-800/60 bg-[#07090e] font-mono text-xs overflow-x-auto rounded-b-lg">
                  {/* Pre context */}
                  {frame.pre_context?.map((line, pIdx) => {
                    const lineNum = frame.lineno - (frame.pre_context?.length || 0) + pIdx;
                    return (
                      <div key={`pre-${pIdx}`} className="flex text-slate-500 leading-5">
                        <span className="w-10 select-none text-right pr-4 text-slate-600">
                          {lineNum}
                        </span>
                        <pre className="whitespace-pre">{line}</pre>
                      </div>
                    );
                  })}

                  {/* Highlighted error line */}
                  <div className="flex bg-rose-500/15 text-rose-200 border-l-2 border-rose-500 -ml-3 pl-2.5 py-0.5 leading-5 my-0.5">
                    <span className="w-10 select-none text-right pr-4 font-bold text-rose-400">
                      {frame.lineno}
                    </span>
                    <pre className="whitespace-pre font-semibold">{frame.context_line}</pre>
                  </div>

                  {/* Post context */}
                  {frame.post_context?.map((line, poIdx) => {
                    const lineNum = frame.lineno + 1 + poIdx;
                    return (
                      <div key={`post-${poIdx}`} className="flex text-slate-500 leading-5">
                        <span className="w-10 select-none text-right pr-4 text-slate-600">
                          {lineNum}
                        </span>
                        <pre className="whitespace-pre">{line}</pre>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
