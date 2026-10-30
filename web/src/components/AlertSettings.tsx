import React, { useState, useEffect } from 'react';
import { AlertConfig } from '../types';
import {
  Bell,
  Plus,
  Trash2,
  Send,
  CheckCircle2,
  AlertTriangle,
  Hash,
  MessageSquare,
  Globe,
} from 'lucide-react';

interface AlertSettingsProps {
  projectId: number;
}

export const AlertSettings: React.FC<AlertSettingsProps> = ({ projectId }) => {
  const [configs, setConfigs] = useState<AlertConfig[]>([]);
  const [channel, setChannel] = useState<'slack' | 'discord' | 'telegram' | 'webhook'>('slack');
  const [targetURL, setTargetURL] = useState('');
  const [notifyOnNew, setNotifyOnNew] = useState(true);
  const [notifyOnRegression, setNotifyOnRegression] = useState(true);
  const [testing, setTesting] = useState(false);
  const [feedback, setFeedback] = useState<{ text: string; error?: boolean } | null>(null);

  const fetchConfigs = async () => {
    try {
      const res = await fetch(`/api/v1/alerts?project_id=${projectId}`);
      if (res.ok) {
        const data = await res.json();
        setConfigs(data || []);
      }
    } catch {
      // Ignore
    }
  };

  useEffect(() => {
    fetchConfigs();
  }, [projectId]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!targetURL) return;

    try {
      const res = await fetch(`/api/v1/alerts?project_id=${projectId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: projectId,
          channel,
          target_url: targetURL,
          enabled: true,
          notify_on_new: notifyOnNew,
          notify_on_regression: notifyOnRegression,
        }),
      });

      if (res.ok) {
        setTargetURL('');
        setFeedback({ text: 'Alert destination configured successfully' });
        fetchConfigs();
      }
    } catch {
      setFeedback({ text: 'Failed saving alert configuration', error: true });
    }
  };

  const handleDelete = async (id?: number) => {
    if (!id) return;
    try {
      const res = await fetch(`/api/v1/alerts/${id}`, { method: 'DELETE' });
      if (res.ok) {
        fetchConfigs();
      }
    } catch {
      // Ignore
    }
  };

  const handleTest = async (cfg: AlertConfig) => {
    setTesting(true);
    setFeedback(null);
    try {
      const res = await fetch('/api/v1/alerts/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg),
      });
      const data = await res.json();
      if (res.ok) {
        setFeedback({ text: `Test alert dispatched to ${cfg.channel}: ${data.message}` });
      } else {
        setFeedback({ text: `Test failed: ${data.error}`, error: true });
      }
    } catch (err: any) {
      setFeedback({ text: `Test failed: ${err.message}`, error: true });
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-6 rounded-xl bg-[#0e111a] border border-slate-800">
        <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
          <Bell className="w-5 h-5 text-emerald-400" />
          Alert Destinations & Webhooks
        </h3>
        <p className="text-xs text-slate-400 mt-1">
          Route notifications to your engineering chat channels when an issue is first encountered or when a regression happens.
        </p>

        {feedback && (
          <div
            className={`mt-4 p-3 rounded-lg text-xs font-medium flex items-center gap-2 ${
              feedback.error
                ? 'bg-rose-500/10 border border-rose-500/30 text-rose-300'
                : 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-300'
            }`}
          >
            {feedback.error ? (
              <AlertTriangle className="w-4 h-4 shrink-0" />
            ) : (
              <CheckCircle2 className="w-4 h-4 shrink-0" />
            )}
            <span>{feedback.text}</span>
          </div>
        )}

        {/* Create Destination Form */}
        <form onSubmit={handleSave} className="mt-6 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">
                Channel
              </label>
              <select
                value={channel}
                onChange={(e) => setChannel(e.target.value as any)}
                className="w-full px-3 py-2 text-xs bg-[#121622] border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              >
                <option value="slack">Slack Incoming Webhook</option>
                <option value="discord">Discord Webhook</option>
                <option value="telegram">Telegram Bot</option>
                <option value="webhook">Generic JSON Webhook</option>
              </select>
            </div>

            <div className="md:col-span-3">
              <label className="block text-xs font-semibold text-slate-400 mb-1">
                Webhook URL or Target Endpoint
              </label>
              <input
                type="text"
                placeholder={
                  channel === 'slack'
                    ? 'https://hooks.slack.com/services/...'
                    : channel === 'discord'
                    ? 'https://discord.com/api/webhooks/...'
                    : channel === 'telegram'
                    ? 'https://api.telegram.org/bot<TOKEN>/sendMessage?chat_id=<CHAT_ID>'
                    : 'https://api.yourdomain.com/webhooks/errors'
                }
                value={targetURL}
                onChange={(e) => setTargetURL(e.target.value)}
                className="w-full px-3 py-2 text-xs font-mono bg-[#121622] border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500 placeholder-slate-600"
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 pt-2">
            <div className="flex items-center gap-6 text-xs text-slate-300">
              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={notifyOnNew}
                  onChange={(e) => setNotifyOnNew(e.target.checked)}
                  className="rounded bg-slate-900 border-slate-700 text-emerald-500 focus:ring-0"
                />
                <span>Notify on First-Seen Issues</span>
              </label>

              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={notifyOnRegression}
                  onChange={(e) => setNotifyOnRegression(e.target.checked)}
                  className="rounded bg-slate-900 border-slate-700 text-emerald-500 focus:ring-0"
                />
                <span>Notify on Regressions (Resolved issue reappears)</span>
              </label>
            </div>

            <button
              type="submit"
              className="flex items-center gap-1.5 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Destination
            </button>
          </div>
        </form>
      </div>

      {/* Configured Destinations */}
      <div className="rounded-xl border border-slate-800 bg-[#0e111a] overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800 bg-[#0a0c13] text-xs font-semibold text-slate-300">
          Configured Alert Channels ({configs.length})
        </div>

        {configs.length === 0 ? (
          <div className="py-8 text-center text-xs text-slate-500 italic">
            No alert destinations set up yet. Add one above to receive instant notifications.
          </div>
        ) : (
          <div className="divide-y divide-slate-800/60">
            {configs.map((c) => (
              <div
                key={c.id}
                className="flex items-center justify-between px-4 py-3 hover:bg-slate-800/30 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-slate-800 text-slate-300">
                    {c.channel === 'slack' && <Hash className="w-4 h-4 text-[#e01e5a]" />}
                    {c.channel === 'discord' && <MessageSquare className="w-4 h-4 text-[#5865f2]" />}
                    {c.channel === 'telegram' && <Send className="w-4 h-4 text-[#229ed9]" />}
                    {c.channel === 'webhook' && <Globe className="w-4 h-4 text-emerald-400" />}
                  </div>

                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-slate-200 uppercase">
                        {c.channel}
                      </span>
                      {c.enabled && (
                        <span className="px-1.5 py-0.2 text-[10px] rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                          Active
                        </span>
                      )}
                    </div>
                    <p className="font-mono text-xs text-slate-500 truncate max-w-md">
                      {c.target_url}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleTest(c)}
                    disabled={testing}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                  >
                    <Send className="w-3 h-3 text-emerald-400" />
                    <span>Send Test</span>
                  </button>

                  <button
                    onClick={() => handleDelete(c.id)}
                    className="p-1.5 rounded-md text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
