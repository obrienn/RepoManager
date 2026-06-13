export interface RepoSummary {
  id: number;
  name: string;
  owner: string;
  github_url: string;
  description: string | null;
  license: string | null;
  default_branch: string;
  last_updated: string | null;
  added_date: string;
  needs_attention: boolean;
  topics: string[];
  tags: Tag[];
  languages: Language[];
  latest_release: Release | null;
}

export interface Repository {
  id: number;
  name: string;
  owner: string;
  github_url: string;
  local_path: string;
  description: string | null;
  license: string | null;
  default_branch: string;
  last_updated: string | null;
  added_date: string;
  total_size_bytes: number;
  needs_attention: boolean;
  attention_reason: string | null;
}

export interface Language {
  language_name: string;
  percentage: number;
  bytes: number;
}

export interface Release {
  tag_name: string;
  published_date: string;
  is_prerelease: boolean;
}

export interface Tag {
  id: number;
  name: string;
  color: string;
}

export interface Metrics {
  total_repos: number;
  need_attention: number;
  total_size_gb: number;
  next_update_time: string;
}

const BASE = '/api';

async function req<T>(url: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, opts);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || res.statusText);
  }
  return res.json();
}

export async function listRepos(params?: Record<string, string>): Promise<RepoSummary[]> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return req<RepoSummary[]>(`/repos${qs}`);
}

export async function getRepo(id: number): Promise<Repository> {
  return req<Repository>(`/repos/${id}`);
}

export async function addRepo(data: { name: string; owner: string; github_url: string }): Promise<Repository> {
  return req<Repository>('/repos', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
}

export async function deleteRepo(id: number): Promise<void> {
  await req('/repos/' + id, { method: 'DELETE' });
}

export async function getReadme(id: number): Promise<string> {
  const res = await fetch(BASE + `/repos/${id}/readme`);
  if (!res.ok) throw new Error('Failed to load README');
  return res.text();
}

export async function updateRepo(id: number): Promise<void> {
  await req(`/repos/${id}/update`, { method: 'POST' });
}

export async function updateAllRepos(): Promise<void> {
  await req('/repos/update-all', { method: 'POST' });
}

export async function recloneRepo(id: number): Promise<void> {
  await req(`/repos/${id}/reclone`, { method: 'POST' });
}

export async function getMetrics(): Promise<Metrics> {
  return req<Metrics>('/metrics');
}

export async function listTags(): Promise<Tag[]> {
  return req<Tag[]>('/tags');
}

export async function createTag(name: string, color: string): Promise<Tag> {
  return req<Tag>('/tags', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, color }),
  });
}

export async function updateTag(id: number, name: string, color: string): Promise<void> {
  await req(`/tags/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, color }),
  });
}

export async function deleteTag(id: number): Promise<void> {
  await req(`/tags/${id}`, { method: 'DELETE' });
}

export async function getRepoTags(id: number): Promise<Tag[]> {
  return req<Tag[]>(`/repos/${id}/tags`);
}

export async function setRepoTags(id: number, tagIds: number[]): Promise<void> {
  await req(`/repos/${id}/tags`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tag_ids: tagIds }),
  });
}
