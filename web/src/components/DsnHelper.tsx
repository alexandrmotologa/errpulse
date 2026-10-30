import React, { useState } from 'react';
import { Project } from '../types';
import { Key, Copy, Check } from 'lucide-react';

interface DsnHelperProps {
  project?: Project | null;
}

export const DsnHelper: React.FC<DsnHelperProps> = ({ project }) => {
  const [copied, setCopied] = useState(false);
  const [activeTab, setActiveTab] = useState<'browser' | 'node' | 'python' | 'go'>('browser');

  const origin = window.location.origin;
  const projectKey = project?.public_key || 'errpulse_default_key';
  const projectId = project?.id || 1;
  const dsn = `${origin.replace('http://', 'http://' + projectKey + '@')}/${projectId}`;

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const snippets = {
    browser: `import * as Sentry from "@sentry/browser";

Sentry.init({
  dsn: "${dsn}",
  release: "my-app@1.0.0",
  environment: "production",
});`,
    node: `import * as Sentry from "@sentry/node";

Sentry.init({
  dsn: "${dsn}",
  release: "backend@1.0.0",
  environment: "production",
});`,
    python: `import sentry_sdk

sentry_sdk.init(
    dsn="${dsn}",
    release="api@2.4.0",
    environment="production",
)`,
    go: `package main

import (
    "time"
    "github.com/getsentry/sentry-go"
)

func main() {
    sentry.Init(sentry.ClientOptions{
        Dsn: "${dsn}",
        Release: "service@1.2.0",
        Environment: "production",
    })
    defer sentry.Flush(2 * time.Second)
}`,
  };

  return (
    <div className="space-y-6">
      <div className="p-6 rounded-xl bg-[#0e111a] border border-slate-800 space-y-4">
        <div>
          <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
            <Key className="w-5 h-5 text-emerald-400" />
            Project DSN & SDK Configuration
          </h3>
          <p className="text-xs text-slate-400 mt-1">
            ErrPulse is 100% protocol-compatible with Sentry. Simply update your application's DSN URL to point to this instance.
          </p>
        </div>

        {/* DSN Copy Box */}
        <div className="flex items-center justify-between gap-3 p-3 rounded-lg bg-[#07090e] border border-slate-700/80 font-mono text-xs">
          <span className="text-emerald-400 truncate select-all">{dsn}</span>
          <button
            onClick={() => copyToClipboard(dsn)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors shrink-0"
          >
            {copied ? (
              <>
                <Check className="w-3.5 h-3.5 text-emerald-400" />
                <span>Copied</span>
              </>
            ) : (
              <>
                <Copy className="w-3.5 h-3.5 text-slate-400" />
                <span>Copy DSN</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Snippet Tabs */}
      <div className="rounded-xl border border-slate-800 bg-[#0e111a] overflow-hidden">
        <div className="flex items-center gap-1 px-4 pt-3 border-b border-slate-800 bg-[#0a0c13]">
          <button
            onClick={() => setActiveTab('browser')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'browser'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            JavaScript / React
          </button>
          <button
            onClick={() => setActiveTab('node')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'node'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Node.js / Next.js
          </button>
          <button
            onClick={() => setActiveTab('python')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'python'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Python
          </button>
          <button
            onClick={() => setActiveTab('go')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'go'
                ? 'border-emerald-400 text-emerald-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Go
          </button>
        </div>

        <div className="p-4 relative bg-[#07090e]">
          <button
            onClick={() => copyToClipboard(snippets[activeTab])}
            className="absolute right-6 top-6 flex items-center gap-1.5 px-2.5 py-1 text-xs rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
          >
            <Copy className="w-3.5 h-3.5" />
            <span>Copy Code</span>
          </button>
          <pre className="font-mono text-xs text-slate-300 overflow-x-auto p-2">
            {snippets[activeTab]}
          </pre>
        </div>
      </div>
    </div>
  );
};
