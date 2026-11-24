import React, { useCallback, useEffect, useMemo, useState } from 'react';

// Configs tab: what basecamp keeps saved in the private dotfiles repo.
// Data comes from `basecamp serve` (GET /api/dotfiles, POST /api/dotfiles/save).

const STATUS = {
  synced:  { label: 'In sync', color: 'var(--green)', bg: 'rgba(52,211,153,0.1)' },
  changed: { label: 'Changed', color: 'var(--yellow)', bg: 'rgba(251,191,36,0.1)' },
  new:     { label: 'Not saved', color: 'var(--accent)', bg: 'var(--accent-bg)' },
  blocked: { label: 'Refused', color: 'var(--red)', bg: 'rgba(248,113,113,0.1)' },
  missing: { label: 'Gone', color: 'var(--fg2)', bg: 'rgba(161,161,170,0.1)' },
  error:   { label: 'Skipped', color: 'var(--fg2)', bg: 'rgba(161,161,170,0.1)' },
};

const STATUS_HELP = {
  synced: 'Matches the last save.',
  changed: 'Edited since the last save.',
  new: 'On this Mac, not saved yet.',
  blocked: 'Looks like it holds a secret, so it is not copied.',
  missing: 'Saved, but no longer on this Mac. Restore brings it back.',
  error: 'Not copied.',
};

const mono = "'JetBrains Mono', monospace";

export async function fetchDotfiles() {
  const r = await fetch('/api/dotfiles', { cache: 'no-store' });
  if (!r.ok) throw new Error(`basecamp API answered ${r.status}`);
  return r.json();
}

export async function saveDotfiles() {
  const r = await fetch('/api/dotfiles/save', { method: 'POST', headers: { 'X-Basecamp': '1' } });
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error || `save failed (${r.status})`);
  return body;
}

export function groupFiles(files) {
  const groups = [];
  const byName = {};
  for (const f of files || []) {
    if (!byName[f.group]) {
      byName[f.group] = { name: f.group, files: [] };
      groups.push(byName[f.group]);
    }
    byName[f.group].files.push(f);
  }
  return groups;
}

export function timeAgo(iso, now = Date.now()) {
  if (!iso) return 'never';
  const mins = Math.round((now - new Date(iso).getTime()) / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs} h ago`;
  return `${Math.round(hrs / 24)} d ago`;
}

function Badge({ status }) {
  const s = STATUS[status] || STATUS.error;
  return (
    <span style={{
      fontSize: 10, fontWeight: 700, letterSpacing: '0.05em', textTransform: 'uppercase',
      padding: '3px 0', borderRadius: 4, color: s.color, background: s.bg, width: 74,
      display: 'inline-flex', justifyContent: 'center',
    }}>{s.label}</span>
  );
}

function Diff({ text }) {
  return (
    <pre style={{
      margin: '0 16px 12px', padding: '10px 12px', background: 'var(--bg)', borderRadius: 7,
      border: '1px solid var(--border)', fontSize: 11.5, lineHeight: 1.55, fontFamily: mono,
      overflow: 'auto', maxHeight: 420, whiteSpace: 'pre',
    }}>
      {text.split('\n').map((line, i) => {
        const color = line.startsWith('+') ? 'var(--green)'
          : line.startsWith('-') ? 'var(--red)'
          : line.startsWith('@@') ? 'var(--accent)' : 'var(--fg2)';
        return <div key={i} style={{ color }}>{line || ' '}</div>;
      })}
    </pre>
  );
}

function FileRow({ f, open, onToggle, last }) {
  const canOpen = Boolean(f.diff) || f.secrets?.length > 0 || f.error;
  return (
    <div style={{ borderBottom: last ? 'none' : '1px solid var(--border)' }}>
      <div
        onClick={canOpen ? onToggle : undefined}
        style={{
          display: 'grid', gridTemplateColumns: 'minmax(160px, 1.2fr) 84px minmax(180px, 2fr) 20px',
          gap: 10, padding: '10px 16px', alignItems: 'center', cursor: canOpen ? 'pointer' : 'default',
        }}
        onMouseEnter={e => { e.currentTarget.style.background = 'var(--accent-hover)'; }}
        onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; }}
      >
        <div style={{ overflow: 'hidden' }}>
          <div style={{ fontFamily: mono, fontSize: 12.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{f.label}</div>
          <div style={{ fontSize: 11, color: 'var(--fg3)', marginTop: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {f.generated ? 'generated' : f.source} {'→'} {f.repoPath}
          </div>
        </div>
        <div style={{ display: 'flex', justifyContent: 'center' }}><Badge status={f.status} /></div>
        <div style={{ fontSize: 12, color: 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {f.status === 'blocked'
            ? f.secrets.map(s => `line ${s.line}: ${s.rule}`).join(', ')
            : f.error || STATUS_HELP[f.status]}
        </div>
        <div style={{ color: 'var(--fg3)', fontSize: 11 }}>{canOpen ? (open ? '▾' : '▸') : ''}</div>
      </div>
      {open && f.status === 'blocked' && (
        <div style={{ margin: '0 16px 10px', fontSize: 12, color: 'var(--fg2)', lineHeight: 1.6 }}>
          Move the value out of the file (a keychain lookup or <code style={{ fontFamily: mono }}>$VAR</code> from
          a file basecamp does not save), or add <code style={{ fontFamily: mono }}>basecamp:allow</code> to
          the line if it is not really a secret.
        </div>
      )}
      {open && f.diff && <Diff text={f.diff} />}
    </div>
  );
}

export default function ConfigsView() {
  const [status, setStatus] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [lastResult, setLastResult] = useState(null);
  const [openRows, setOpenRows] = useState({});
  const [onlyPending, setOnlyPending] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    fetchDotfiles()
      .then(s => { setStatus(s); setError(null); })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  const save = () => {
    setSaving(true);
    setLastResult(null);
    saveDotfiles()
      .then(res => { setLastResult(res); load(); })
      .catch(e => setLastResult({ error: e.message }))
      .finally(() => setSaving(false));
  };

  const files = useMemo(() => {
    const all = status?.files || [];
    return onlyPending ? all.filter(f => f.status !== 'synced') : all;
  }, [status, onlyPending]);
  const groups = useMemo(() => groupFiles(files), [files]);
  const counts = status?.counts || {};
  const pending = (counts.changed || 0) + (counts.new || 0);

  if (error && !status) {
    return (
      <div className="slide-in" style={{ padding: 32, maxWidth: 640 }}>
        <h2 style={{ fontSize: 19, fontWeight: 700, marginBottom: 8 }}>🗂 Configs</h2>
        <p style={{ fontSize: 13, color: 'var(--fg3)', lineHeight: 1.6, marginBottom: 16 }}>
          The Configs tab talks to <code style={{ fontFamily: mono }}>basecamp serve</code>, which is not reachable ({error}).
          The same status is on the command line:
        </p>
        <pre style={{ background: 'var(--card)', border: '1px solid var(--border)', borderRadius: 8, padding: 16, fontSize: 12, color: 'var(--fg2)', fontFamily: mono }}>
{`basecamp dotfiles status
basecamp dotfiles diff
basecamp dotfiles save --push`}
        </pre>
      </div>
    );
  }

  return (
    <div className="slide-in">
      <div style={{ padding: '24px 32px 0' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 18, gap: 16 }}>
          <div>
            <h2 style={{ fontSize: 19, fontWeight: 700, letterSpacing: '-0.02em', marginBottom: 4 }}>🗂 Configs</h2>
            <p style={{ fontSize: 13, color: 'var(--fg3)' }}>
              Saved to {status?.remote ? <span style={{ fontFamily: mono }}>{status.remote.replace(/^.*github\.com[:/]/, '').replace(/\.git$/, '')}</span> : 'a local repo'}
              {' · '}last save {timeAgo(status?.lastSave)}
              {status?.unpushed > 0 && <span style={{ color: 'var(--yellow)' }}> · {status.unpushed} not pushed</span>}
            </p>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={load} disabled={loading} style={btn(false)}>{loading ? 'Checking…' : 'Refresh'}</button>
            <button onClick={save} disabled={saving || !status?.initialized || (pending === 0 && !status?.unpushed)} style={btn(true, saving || pending === 0 && !status?.unpushed)}>
              {saving ? 'Saving…' : pending > 0 ? `Save ${pending}${status?.remote ? ' and push' : ''}` : status?.unpushed ? 'Push' : 'Nothing to save'}
            </button>
          </div>
        </div>

        {!status?.initialized && status && (
          <Notice color="var(--yellow)">
            No dotfiles repo at <code style={{ fontFamily: mono }}>{status.repo}</code>. Run <code style={{ fontFamily: mono }}>basecamp dotfiles init &lt;git-url&gt;</code>.
          </Notice>
        )}
        {lastResult && <SaveNotice res={lastResult} />}

        <div style={{
          display: 'flex', gap: 18, padding: '12px 16px', background: 'var(--card)', borderRadius: 9,
          border: '1px solid var(--border)', marginBottom: 14, fontSize: 12, alignItems: 'center', flexWrap: 'wrap',
        }}>
          {['synced', 'changed', 'new', 'blocked', 'missing'].map(k => (
            <span key={k}><span style={{ color: STATUS[k].color }}>●</span> <strong>{counts[k] || 0}</strong> <span style={{ color: 'var(--fg3)' }}>{STATUS[k].label.toLowerCase()}</span></span>
          ))}
          <label style={{ marginLeft: 'auto', color: 'var(--fg3)', display: 'flex', gap: 6, alignItems: 'center', cursor: 'pointer' }}>
            <input type="checkbox" checked={onlyPending} onChange={e => setOnlyPending(e.target.checked)} />
            Hide in-sync files
          </label>
        </div>
      </div>

      <div style={{ padding: '0 32px 48px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {groups.length === 0 && (
          <div style={{ padding: 28, textAlign: 'center', color: 'var(--fg3)', fontSize: 13, background: 'var(--card)', borderRadius: 9, border: '1px solid var(--border)' }}>
            {!status ? 'Checking your configs…' : onlyPending ? 'Everything matches the last save.' : 'No configs found.'}
          </div>
        )}
        {groups.map(g => (
          <div key={g.name}>
            <div style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', margin: '0 0 6px 2px' }}>{g.name}</div>
            <div style={{ background: 'var(--card)', borderRadius: 9, border: '1px solid var(--border)', overflow: 'hidden' }}>
              {g.files.map((f, i) => (
                <FileRow key={f.repoPath} f={f} last={i === g.files.length - 1}
                  open={Boolean(openRows[f.repoPath])}
                  onToggle={() => setOpenRows(o => ({ ...o, [f.repoPath]: !o[f.repoPath] }))} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function SaveNotice({ res }) {
  if (res.error) return <Notice color="var(--red)">Save failed: {res.error}</Notice>;
  const parts = [];
  parts.push(res.saved?.length ? `Saved ${res.saved.length} file${res.saved.length === 1 ? '' : 's'}` : 'Nothing new to save');
  if (res.pushed) parts.push('pushed');
  if (res.pushError) parts.push(`push failed: ${res.pushError}`);
  return (
    <Notice color={res.pushError ? 'var(--yellow)' : 'var(--green)'}>
      {parts.join(', ')}.
      {res.skipped?.length > 0 && <> Skipped {res.skipped.map(s => s.repoPath).join(', ')}.</>}
    </Notice>
  );
}

function Notice({ color, children }) {
  return (
    <div style={{
      fontSize: 12.5, color: 'var(--fg2)', padding: '10px 14px', borderRadius: 8, marginBottom: 12,
      background: 'var(--card)', border: '1px solid var(--border)', borderLeft: `3px solid ${color}`, lineHeight: 1.5,
    }}>{children}</div>
  );
}

function btn(primary, dim) {
  return {
    padding: '8px 14px', borderRadius: 7, fontSize: 12, fontWeight: 600, fontFamily: 'inherit',
    cursor: dim ? 'default' : 'pointer', whiteSpace: 'nowrap',
    border: primary ? 'none' : '1px solid var(--border2)',
    background: primary ? 'var(--accent)' : 'var(--card2)',
    color: primary ? '#000' : 'var(--fg)', opacity: dim ? 0.5 : 1,
  };
}
