export type IssueStatus = 'unresolved' | 'resolved' | 'ignored';

export interface Issue {
  id: number;
  project_id: number;
  fingerprint: string;
  title: string;
  culprit: string;
  type: string;
  status: IssueStatus;
  first_seen_at: string;
  last_seen_at: string;
  event_count: number;
  user_count: number;
  last_release: string;
  sparkline?: number[];
}

export interface SentryFrame {
  filename: string;
  function?: string;
  module?: string;
  lineno: number;
  colno?: number;
  abs_path?: string;
  context_line?: string;
  pre_context?: string[];
  post_context?: string[];
  in_app?: boolean;
}

export interface SentryException {
  type: string;
  value: string;
  stacktrace?: {
    frames: SentryFrame[];
  };
}

export interface SentryBreadcrumb {
  type?: string;
  category?: string;
  message?: string;
  data?: Record<string, any>;
  level?: string;
  timestamp?: number | string;
}

export interface EventDetail {
  id: string;
  issue_id: number;
  project_id: number;
  timestamp: string;
  environment: string;
  release: string;
  platform: string;
  message: string;
  user_json?: string;
  tags_json?: string;
  extra_json?: string;
  breadcrumbs_json?: string;
  stacktrace_json?: string;
  raw_payload?: string;
  created_at: string;
}

export interface Project {
  id: number;
  name: string;
  slug: string;
  public_key: string;
  secret_key?: string;
  created_at: string;
}

export interface StatsSummary {
  total_events: number;
  unresolved_issues: number;
  events_24h: number;
  total_projects: number;
}

export interface AlertConfig {
  id?: number;
  project_id: number;
  channel: 'slack' | 'discord' | 'telegram' | 'webhook';
  target_url: string;
  enabled: boolean;
  notify_on_new: boolean;
  notify_on_regression: boolean;
}

export interface ReleaseArtifact {
  id: number;
  project_id: number;
  release: string;
  filename: string;
  content_type: string;
  size?: number;
  created_at: string;
}
