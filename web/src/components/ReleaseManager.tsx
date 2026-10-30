import React, { useState, useEffect } from 'react';
import { ReleaseArtifact, ReleaseHealth } from '../types';
import {
  UploadCloud,
  FileCode,
  CheckCircle2,
  Trash2,
  Package,
  Terminal,
  ShieldCheck,
  Users,
} from 'lucide-react';

interface ReleaseManagerProps {
  projectId: number;
}

export const ReleaseManager: React.FC<ReleaseManagerProps> = ({ projectId }) => {
  const [releases, setReleases] = useState<string[]>([]);
  const [selectedRelease, setSelectedRelease] = useState<string>('');
  const [healthMap, setHealthMap] = useState<Record<string, ReleaseHealth>>({});
  const [artifacts, setArtifacts] = useState<ReleaseArtifact[]>([]);
  const [newReleaseTag, setNewReleaseTag] = useState('');
  const [uploading, setUploading] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; error?: boolean } | null>(null);

  const fetchReleases = async () => {
    try {
      const res = await fetch(`/api/v1/releases?project_id=${projectId}`);
      if (res.ok) {
        const data = await res.json();
        setReleases(data || []);
        if (!selectedRelease && data && data.length > 0) {
          setSelectedRelease(data[0]);
        }
      }
      const healthRes = await fetch(`/api/v1/releases/health?project_id=${projectId}`);
      if (healthRes.ok) {
        const hList: ReleaseHealth[] = await healthRes.json();
        const map: Record<string, ReleaseHealth> = {};
        hList.forEach((h) => {
          map[h.release] = h;
        });
        setHealthMap(map);
      }
    } catch {
      // Ignore
    }
  };

  const fetchArtifacts = async (rel: string) => {
    if (!rel) return;
    try {
      const res = await fetch(`/api/v1/releases/${encodeURIComponent(rel)}/artifacts?project_id=${projectId}`);
      if (res.ok) {
        const data = await res.json();
        setArtifacts(data || []);
      }
    } catch {
      // Ignore
    }
  };

  useEffect(() => {
    fetchReleases();
  }, [projectId]);

  useEffect(() => {
    if (selectedRelease) {
      fetchArtifacts(selectedRelease);
    }
  }, [selectedRelease, projectId]);

  const handleFileUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const releaseTarget = selectedRelease || newReleaseTag.trim();
    if (!releaseTarget) {
      setStatusMsg({ text: 'Please specify or select a release tag first.', error: true });
      return;
    }

    setUploading(true);
    setStatusMsg(null);

    let successCount = 0;
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const formData = new FormData();
      formData.append('file', file);
      formData.append('filename', file.name);

      try {
        const res = await fetch(`/api/v1/releases/${encodeURIComponent(releaseTarget)}/artifacts?project_id=${projectId}`, {
          method: 'POST',
          body: formData,
        });
        if (res.ok) {
          successCount++;
        }
      } catch {
        // Continue
      }
    }

    setUploading(false);
    setStatusMsg({ text: `Successfully uploaded ${successCount} artifact(s) for ${releaseTarget}` });
    if (!selectedRelease && newReleaseTag) {
      setSelectedRelease(newReleaseTag);
      setNewReleaseTag('');
    }
    fetchReleases();
    fetchArtifacts(releaseTarget);
  };

  const deleteArtifact = async (id: number) => {
    try {
      const res = await fetch(`/api/v1/artifacts/${id}`, { method: 'DELETE' });
      if (res.ok && selectedRelease) {
        fetchArtifacts(selectedRelease);
      }
    } catch {
      // Ignore
    }
  };

  return (
    <div className="space-y-6">
      {/* Overview & Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-6 rounded-xl bg-[#0e111a] border border-slate-800">
        <div>
          <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
            <Package className="w-5 h-5 text-emerald-400" />
            Release & Source Map Manager
          </h3>
          <p className="text-xs text-slate-400 mt-1">
            Upload JavaScript and TypeScript source maps (`.map`) to translate minified production stacks back into real files and line numbers.
          </p>
        </div>

        {/* Release Selector */}
        <div className="flex items-center gap-2">
          {releases.length > 0 && (
            <select
              value={selectedRelease}
              onChange={(e) => setSelectedRelease(e.target.value)}
              className="px-3 py-1.5 text-xs font-mono bg-[#121622] border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
            >
              {releases.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          )}

          <div className="flex items-center gap-2">
            <input
              type="text"
              placeholder="Or new tag (e.g. webapp@1.0.0)"
              value={newReleaseTag}
              onChange={(e) => setNewReleaseTag(e.target.value)}
              className="px-3 py-1.5 text-xs font-mono bg-[#121622] border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 placeholder-slate-600"
            />
          </div>
        </div>
      </div>

      {/* Selected Release Health Badges */}
      {selectedRelease && healthMap[selectedRelease] && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 p-4 rounded-xl bg-[#0a0c13] border border-slate-800 font-mono text-xs">
          <div className="space-y-1">
            <span className="text-slate-500 text-[10px] uppercase font-bold flex items-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              Crash-Free Sessions
            </span>
            <div className="text-base font-bold text-emerald-400">
              {healthMap[selectedRelease].crash_free_sessions_rate}%
            </div>
          </div>

          <div className="space-y-1">
            <span className="text-slate-500 text-[10px] uppercase font-bold flex items-center gap-1">
              <Users className="w-3.5 h-3.5 text-sky-400" />
              Crash-Free Users
            </span>
            <div className="text-base font-bold text-sky-400">
              {healthMap[selectedRelease].crash_free_users_rate}%
            </div>
          </div>

          <div className="space-y-1">
            <span className="text-slate-500 text-[10px] uppercase font-bold">Total Sessions</span>
            <div className="text-base font-bold text-slate-200">
              {healthMap[selectedRelease].total_sessions.toLocaleString()}
            </div>
          </div>

          <div className="space-y-1">
            <span className="text-slate-500 text-[10px] uppercase font-bold">Crashed Sessions</span>
            <div className="text-base font-bold text-rose-400">
              {healthMap[selectedRelease].crashed_sessions.toLocaleString()}
            </div>
          </div>
        </div>
      )}

      {statusMsg && (
        <div
          className={`p-3 rounded-lg text-xs font-medium flex items-center gap-2 ${
            statusMsg.error
              ? 'bg-rose-500/10 border border-rose-500/30 text-rose-300'
              : 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-300'
          }`}
        >
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          <span>{statusMsg.text}</span>
        </div>
      )}

      {/* Drag & Drop Upload Zone */}
      <div className="border-2 border-dashed border-slate-800 hover:border-emerald-500/50 rounded-xl p-8 text-center bg-[#0a0c13] transition-colors relative">
        <input
          type="file"
          multiple
          accept=".map,.js"
          onChange={(e) => handleFileUpload(e.target.files)}
          className="absolute inset-0 opacity-0 cursor-pointer w-full h-full"
          disabled={uploading}
        />
        <div className="flex flex-col items-center justify-center space-y-2 pointer-events-none">
          <UploadCloud className="w-10 h-10 text-emerald-400/80 mb-1" />
          <p className="text-sm font-semibold text-slate-200">
            {uploading ? 'Uploading artifacts...' : 'Drag and drop .map files here, or browse'}
          </p>
          <p className="text-xs text-slate-500">
            Target release:{' '}
            <span className="font-mono text-emerald-400 font-semibold">
              {selectedRelease || newReleaseTag || '<specify tag>'}
            </span>
          </p>
        </div>
      </div>

      {/* Uploaded Artifacts List */}
      <div className="rounded-xl border border-slate-800 bg-[#0e111a] overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800 bg-[#0a0c13] flex items-center justify-between text-xs font-semibold text-slate-300">
          <span>Uploaded Artifacts ({artifacts.length})</span>
          {selectedRelease && <span className="font-mono text-emerald-400">{selectedRelease}</span>}
        </div>

        {artifacts.length === 0 ? (
          <div className="py-8 text-center text-xs text-slate-500 italic">
            No source map files uploaded for this release yet.
          </div>
        ) : (
          <div className="divide-y divide-slate-800/60">
            {artifacts.map((a) => (
              <div
                key={a.id}
                className="flex items-center justify-between px-4 py-2.5 text-xs font-mono hover:bg-slate-800/30 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <FileCode className="w-4 h-4 text-emerald-400" />
                  <span className="text-slate-200">{a.filename}</span>
                </div>

                <div className="flex items-center gap-4 text-slate-400">
                  <span>{new Date(a.created_at).toLocaleDateString()}</span>
                  <button
                    onClick={() => deleteArtifact(a.id)}
                    title="Delete artifact"
                    className="p-1 rounded text-slate-500 hover:text-rose-400 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* CI/CD sentry-cli hint */}
      <div className="p-4 rounded-xl bg-[#0b0e15] border border-slate-800 space-y-2">
        <h4 className="text-xs font-bold text-slate-300 flex items-center gap-1.5">
          <Terminal className="w-3.5 h-3.5 text-emerald-400" />
          Automate via sentry-cli in CI/CD
        </h4>
        <pre className="p-3 rounded-lg bg-black/60 text-[11px] font-mono text-slate-400 overflow-x-auto">
{`export SENTRY_URL=http://localhost:9988
export SENTRY_AUTH_TOKEN=errpulse_default_key
export SENTRY_ORG=default
export SENTRY_PROJECT=1

sentry-cli releases files webapp@1.0.0 upload-sourcemaps ./dist --url-prefix "~/"`}
        </pre>
      </div>
    </div>
  );
};
