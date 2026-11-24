import React, { useCallback, useEffect, useMemo, useState } from 'react';

// The Jobs tab: every scheduled LaunchAgent, whether its last run worked, and backups that are
// stale or don't reach the offsite image. Data: GET /jobs.json, written by `basecamp jobs`
// (launchd com.basecamp-jobs, every 15 minutes).

const RED = new Set(['failed', 'missed', 'stale', 'unloaded', 'down']);

const STATUS = {
  ok:       { color: 'var(--green)', bg: 'rgba(52,211,153,0.1)', label: 'OK' },
  running:  { color: 'var(--green)', bg: 'rgba(52,211,153,0.1)', label: 'Running' },
  idle:     { color: 'var(--fg2)', bg: 'rgba(161,161,170,0.08)', label: 'Idle' },
  warning:  { color: 'var(--yellow)', bg: 'rgba(251,191,36,0.1)', label: 'Warning' },
  pending:  { color: 'var(--fg2)', bg: 'rgba(161,161,170,0.08)', label: 'Pending' },
  unknown:  { color: 'var(--fg2)', bg: 'rgba(161,161,170,0.08)', label: 'Unknown' },
  failed:   { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Failed' },
  missed:   { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Missed' },
  stale:    { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Stale' },
  unloaded: { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Unloaded' },
  down:     { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Down' },
};
const statusCfg = s => STATUS[s] || STATUS.unknown;

const MONO = "'JetBrains Mono', monospace";
const card = { background: 'var(--card)', border: '1px solid var(--border)', borderRadius: '0.75rem' };
const sectionTitle = {
  fontSize: 'var(--text-xs)', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)',
};

/** Loads /jobs.json; App uses it for the sidebar badge and passes it to <JobsView>. */
export function useJobs() {
  const [state, setState] = useState({ data: null, error: null, loading: true });
  const load = useCallback(() => {
    setState(s => ({ ...s, loading: true }));
    return fetch('/jobs.json', { cache: 'no-store' })
      .then(r => {
        if (!r.ok) throw new Error(r.status === 404 ? 'No jobs report yet' : `HTTP ${r.status}`);
        return r.json();
      })
      .then(data => setState({ data, error: null, loading: false }))
      .catch(err => setState(s => ({ ...s, error: err.message, loading: false })));
  }, []);
  useEffect(() => {
    load();
    const id = setInterval(load, 60_000);
    return () => clearInterval(id);
  }, [load]);
  return { ...state, reload: load };
}

/** Sidebar entry, styled like the category buttons, with a red count when jobs need attention. */
export function JobsNavButton({ jobs, active, onClick }) {
  const [hov, setHov] = useState(false);
  const red = jobs.data ? jobs.data.jobs.filter(j => j.red).length : 0;
  const warn = jobs.data ? jobs.data.jobs.filter(j => j.status === 'warning').length : 0;
  return (
    <button onClick={onClick} onMouseEnter={() => setHov(true)} onMouseLeave={() => setHov(false)} style={{
      display: 'flex', alignItems: 'center', gap: '0.75rem', width: '100%',
      padding: '0.625rem 0.75rem', borderRadius: '0.5rem', border: 'none', cursor: 'pointer',
      background: active ? 'var(--accent-bg)' : hov ? 'var(--accent-hover)' : 'transparent',
      color: active ? 'var(--accent)' : hov ? 'var(--fg)' : 'var(--fg2)',
      fontSize: 'var(--text-base)', fontWeight: active ? 600 : 400, transition: 'all 0.12s', textAlign: 'left',
      fontFamily: 'inherit', marginBottom: '0.5rem', borderBottom: '1px solid var(--border)',
    }}>
      <span style={{ fontSize: 'var(--text-base)', width: '1.5rem', textAlign: 'center' }}>⏱</span>
      <span style={{ flex: 1 }}>Jobs</span>
      {(red > 0 || warn > 0) && (
        <span style={{
          fontSize: 'var(--text-xs)', fontWeight: 700, minWidth: '1.5rem', height: '1.5rem', padding: '0 0.375rem',
          display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: '0.75rem',
          background: red ? 'rgba(248,113,113,0.12)' : 'rgba(251,191,36,0.12)',
          color: red ? 'var(--red)' : 'var(--yellow)',
        }}>{red || warn}</span>
      )}
    </button>
  );
}

// ---------- time helpers ----------

const toDate = s => (s ? new Date(s) : null);
const hm = d => d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });

function dayLabel(d, now = new Date()) {
  const start = x => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((start(now) - start(d)) / 86_400_000);
  if (diff === 0) return 'today';
  if (diff === 1) return 'yesterday';
  if (diff === -1) return 'tomorrow';
  return d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
}
const when = s => {
  const d = toDate(s);
  return d ? `${dayLabel(d)} ${hm(d)}` : '';
};
function ago(s) {
  const d = toDate(s);
  if (!d) return '';
  const mins = Math.round((Date.now() - d.getTime()) / 60_000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min ago`;
  const h = Math.round(mins / 60);
  return h < 48 ? `${h} h ago` : `${Math.round(h / 24)} days ago`;
}
const minutesOfDay = d => d.getHours() * 60 + d.getMinutes() + d.getSeconds() / 60;
const parseHM = s => {
  const [h, m] = s.split(':').map(Number);
  return h * 60 + m;
};
const fmtHM = m => `${String(Math.floor(m / 60) % 24).padStart(2, '0')}:${String(Math.round(m % 60)).padStart(2, '0')}`;

// ---------- small pieces ----------

function Badge({ status }) {
  const c = statusCfg(status);
  return (
    <span style={{
      fontSize: 'var(--text-xs)', fontWeight: 700, letterSpacing: '0.05em', textTransform: 'uppercase',
      padding: '0.25rem 0', borderRadius: '0.375rem', color: c.color, background: c.bg, width: '5.5rem',
      display: 'inline-flex', justifyContent: 'center', flexShrink: 0,
    }}>{c.label}</span>
  );
}

function Chip({ tone = 'muted', children, title }) {
  const tones = {
    good: ['var(--green)', 'rgba(52,211,153,0.08)'],
    warn: ['var(--yellow)', 'rgba(251,191,36,0.08)'],
    bad: ['var(--red)', 'rgba(248,113,113,0.08)'],
    muted: ['var(--fg2)', 'rgba(161,161,170,0.07)'],
  };
  const [color, bg] = tones[tone];
  return (
    <span title={title} style={{
      fontSize: 'var(--text-sm)', color, background: bg, padding: '2px 0.5rem', borderRadius: '0.375rem', whiteSpace: 'nowrap',
    }}>{children}</span>
  );
}

const OFFSITE = {
  'in-image': ['good', s => `in ${s.offsiteImage}`],
  'next-image': ['muted', () => 'goes in the next offsite image'],
  'not-included': ['warn', () => 'not in the offsite image'],
  unknown: ['muted', () => 'offsite: unknown'],
};

/** One line saying what the last run did. */
function resultLine(job) {
  if (job.report?.summary) return job.report.summary;
  if (job.reasons?.length) return job.reasons[0];
  if (job.status === 'running' && job.kind !== 'service') return 'running now';
  // Prefer the last timestamped line (what the job reported) over trailing noise like pruned paths.
  const tail = (job.log?.tail || []).map(l => l.trim()).filter(Boolean);
  const stamp = /^\[?\d{4}-\d{2}-\d{2}[T ][\d:.]+(Z|[+-]\d{2}:?\d{2})?\]?\s*/;
  const line = [...tail].reverse().find(l => stamp.test(l)) || tail[tail.length - 1];
  if (line) return line.replace(stamp, '');
  return job.log?.error || 'no output';
}

// ---------- night timeline ----------

function NightTimeline({ jobs }) {
  const rows = jobs.filter(j => j.kind === 'calendar' && j.slots?.length);
  const layout = useMemo(() => {
    if (!rows.length) return null;
    const slots = rows.flatMap(j => j.slots.map(parseHM));
    const start = Math.floor(Math.min(...slots) / 60) * 60;
    let end = Math.max(...slots) + 30;
    for (const j of rows) {
      const s = toDate(j.runStart), e = toDate(j.runEnd);
      if (s && e && e - s < 6 * 3600_000) {
        const em = minutesOfDay(e) + (e.getDate() !== s.getDate() ? 1440 : 0);
        if (em > end && em - start < 8 * 60) end = em + 10;
      }
    }
    end = Math.ceil(end / 30) * 30;
    return { start, end };
  }, [rows]);
  if (!layout) return null;
  const { start, end } = layout;
  const span = end - start;
  const pct = m => `${((m - start) / span) * 100}%`;
  const ticks = [];
  for (let m = start; m <= end; m += 30) ticks.push(m);

  return (
    <div style={{ ...card, padding: '1rem 1.25rem 0.75rem', marginBottom: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
        <span style={sectionTitle}>Last night, {fmtHM(start)} to {fmtHM(end)}</span>
        <span style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)', display: 'flex', gap: '1rem' }}>
          <span><span style={{ display: 'inline-block', width: '0.5rem', height: '0.5rem', border: '1.5px solid var(--fg3)', borderRadius: 2, marginRight: '0.375rem' }} />scheduled</span>
          <span><span style={{ display: 'inline-block', width: '1rem', height: '0.375rem', background: 'var(--green)', borderRadius: 2, marginRight: '0.375rem', verticalAlign: 'middle' }} />ran</span>
        </span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '15rem 1fr', rowGap: '0.375rem', alignItems: 'center' }}>
        <div />
        <div style={{ position: 'relative', height: '1rem' }}>
          {ticks.map(m => (
            <span key={m} style={{
              position: 'absolute', left: pct(m), transform: 'translateX(-50%)',
              fontSize: 'var(--text-xs)', color: 'var(--fg4)', fontFamily: MONO,
            }}>{m % 60 === 0 ? fmtHM(m) : ''}</span>
          ))}
        </div>
        {rows.map(j => {
          const color = statusCfg(j.status).color;
          const s = toDate(j.runStart) || toDate(j.lastRun);
          const e = toDate(j.runEnd) || toDate(j.lastRun);
          // Only draw the run if it belongs to the night being shown (its own most recent slot).
          const due = toDate(j.dueSlot);
          const fresh = s && (!due || s >= new Date(due.getTime() - 5 * 60_000)) && Date.now() - s < 30 * 3600_000;
          let barL = null, barW = null;
          const sm = s ? minutesOfDay(s) : 0;
          const inWindow = fresh && sm >= start - 5 && sm <= end;
          if (inWindow) {
            let em = e ? minutesOfDay(e) : sm;
            if (em < sm) em += 1440;
            barL = Math.max(start, Math.min(end, sm));
            barW = Math.max(0.6, Math.min(end, em) - barL);
          }
          const title = inWindow
            ? `${j.label}: ran ${hm(s)}${e && e - s > 60_000 ? ` to ${hm(e)}` : ''} · ${statusCfg(j.status).label}`
            : fresh ? `${j.label}: last ran ${when(j.lastRun)}, outside this window · ${statusCfg(j.status).label}`
            : `${j.label}: no run for the latest slot · ${statusCfg(j.status).label}`;
          return (
            <React.Fragment key={j.label}>
              <div style={{ fontSize: 'var(--text-sm)', fontFamily: MONO, color: 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', paddingRight: '0.75rem' }} title={j.label}>
                {j.label.replace(/^(com|local)\./, '')}
              </div>
              <div title={title} style={{ position: 'relative', height: '1.5rem', borderRadius: '0.375rem', background: 'rgba(255,255,255,0.015)' }}>
                {ticks.map(m => (
                  <span key={m} style={{ position: 'absolute', left: pct(m), top: 0, bottom: 0, width: 1, background: m % 60 === 0 ? 'var(--border2)' : 'var(--border)' }} />
                ))}
                {j.slots.map(sl => (
                  <span key={sl} style={{
                    position: 'absolute', left: pct(parseHM(sl)), top: '0.375rem', width: '0.625rem', height: '0.625rem', marginLeft: '-0.25rem',
                    border: `1.5px solid ${RED.has(j.status) && !inWindow ? color : 'var(--fg3)'}`, borderRadius: 2,
                    borderStyle: RED.has(j.status) && !inWindow ? 'dashed' : 'solid',
                  }} />
                ))}
                {inWindow && (
                  <span style={{
                    position: 'absolute', left: pct(barL), width: `${(barW / span) * 100}%`, minWidth: '0.375rem',
                    top: '0.375rem', height: '0.625rem', borderRadius: '0.25rem', background: color, opacity: 0.9,
                  }} />
                )}
              </div>
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}

// ---------- job rows ----------

function JobRow({ job, open, onToggle }) {
  const c = statusCfg(job.status);
  const lastRun = job.kind === 'service' ? (job.pid ? `pid ${job.pid}` : 'not running') : (when(job.lastRun) || 'no run seen');
  return (
    <div style={{ borderBottom: '1px solid var(--border)' }}>
      <div onClick={onToggle} style={{
        display: 'grid', gridTemplateColumns: '5.5rem minmax(10rem, 1fr) 8rem 9rem minmax(0, 2fr) 1rem',
        gap: '1rem', padding: '0.75rem 1.25rem', alignItems: 'center', cursor: 'pointer',
      }}
        onMouseEnter={e => (e.currentTarget.style.background = 'var(--accent-hover)')}
        onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
        <Badge status={job.status} />
        <div style={{ overflow: 'hidden' }}>
          <div style={{ fontFamily: MONO, fontSize: 'var(--text-base)', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{job.label}</div>
        </div>
        <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{job.schedule}</div>
        <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg2)', whiteSpace: 'nowrap' }} title={job.lastRunSource ? `from ${job.lastRunSource}` : ''}>{lastRun}</div>
        <div style={{ fontSize: 'var(--text-sm)', color: RED.has(job.status) ? c.color : 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {resultLine(job)}
        </div>
        <span style={{ color: 'var(--fg4)', fontSize: 'var(--text-sm)', transform: open ? 'rotate(90deg)' : 'none', transition: 'transform 0.12s' }}>▸</span>
      </div>
      {!open && job.snapshots?.length > 0 && (
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', padding: '0 1.25rem 0.75rem 8rem', marginTop: -2 }}>
          {job.snapshots.map(s => <SnapshotChips key={s.name} snap={s} compact />)}
        </div>
      )}
      {open && <JobDetail job={job} />}
    </div>
  );
}

function SnapshotChips({ snap, compact }) {
  const age = snap.error ? null : snap.ageHours < 36 ? `${Math.round(snap.ageHours)} h old` : `${(snap.ageHours / 24).toFixed(1)} days old`;
  const off = snap.offsite && OFFSITE[snap.offsite];
  return (
    <span style={{ display: 'inline-flex', gap: '0.375rem', alignItems: 'center' }}>
      <Chip tone={snap.stale ? 'bad' : 'good'} title={snap.file || snap.error}>
        {snap.name}: {snap.error ? 'no snapshot' : age}
      </Chip>
      {off && !(compact && snap.offsite === 'in-image') && <Chip tone={off[0]}>{off[1](snap)}</Chip>}
      {off && compact && snap.offsite === 'in-image' && <Chip tone="good" title={snap.offsiteImage}>offsite ✓</Chip>}
    </span>
  );
}

function JobDetail({ job }) {
  const errSet = new Set(job.errors || []);
  const warnSet = new Set(job.warnings || []);
  const kv = (k, v) => v ? (
    <div style={{ display: 'contents' }}>
      <span style={{ color: 'var(--fg3)' }}>{k}</span>
      <span style={{ fontFamily: MONO, fontSize: 'var(--text-sm)', color: 'var(--fg2)', wordBreak: 'break-all' }}>{v}</span>
    </div>
  ) : null;
  const exit = job.lastExit === undefined || job.lastExit === null ? 'never exited since load' : String(job.lastExit);
  return (
    <div className="slide-in" style={{ padding: '0.375rem 1.25rem 1.25rem 8rem', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {(job.reasons?.length > 0 || job.notes?.length > 0) && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem' }}>
          {job.reasons.map((r, i) => <div key={`r${i}`} style={{ fontSize: 'var(--text-base)', color: statusCfg(job.status).color }}>• {r}</div>)}
          {job.notes.map((n, i) => (
            <div key={`n${i}`} style={{ fontSize: 'var(--text-base)', color: n.level === 'warn' ? 'var(--yellow)' : 'var(--fg3)' }}>• {n.text}</div>
          ))}
        </div>
      )}
      {job.report && (
        <div style={{ fontSize: 'var(--text-base)', color: 'var(--fg2)' }}>
          <span style={{ color: 'var(--fg3)' }}>Status file: </span>
          {job.report.ok === false ? 'failed' : 'ok'}{job.report.summary ? `, ${job.report.summary}` : ''}
          {job.report.finished ? ` (finished ${when(job.report.finished)})` : ''}
        </div>
      )}
      {job.snapshots?.length > 0 && (
        <div>
          <div style={{ ...sectionTitle, marginBottom: '0.5rem' }}>Backups</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'auto auto 1fr', gap: '0.375rem 1rem', fontSize: 'var(--text-sm)', alignItems: 'center' }}>
            {job.snapshots.map(s => {
              const off = s.offsite && OFFSITE[s.offsite];
              return (
                <React.Fragment key={s.name}>
                  <span style={{ fontFamily: MONO, color: 'var(--fg2)' }}>{s.file || s.name}</span>
                  <Chip tone={s.stale ? 'bad' : 'good'}>{s.error ? s.error : `${when(s.at)}${s.source === 'log' ? ' (from log)' : ''}`}</Chip>
                  <span>{off ? <Chip tone={off[0]}>{off[1](s)}</Chip> : null}</span>
                </React.Fragment>
              );
            })}
          </div>
        </div>
      )}
      <div style={{ display: 'grid', gridTemplateColumns: '7rem 1fr', gap: '0.375rem 1rem', fontSize: 'var(--text-sm)' }}>
        {kv('Command', job.command)}
        {kv('Runs', `${job.runs} since load · last exit ${exit}${job.pid ? ` · pid ${job.pid}` : ''}`)}
        {kv('Last run', job.lastRun ? `${when(job.lastRun)} (${job.lastRunSource})${job.runStart && job.runEnd && job.runEnd !== job.runStart ? `, ${hm(new Date(job.runStart))} to ${hm(new Date(job.runEnd))}` : ''}` : '')}
        {kv('Next run', when(job.nextRun))}
        {kv('Plist', job.plist)}
      </div>
      {job.log && (
        <div>
          <div style={{ ...sectionTitle, marginBottom: '0.5rem', display: 'flex', justifyContent: 'space-between', gap: '0.75rem' }}>
            <span>Log tail</span>
            <span style={{ fontFamily: MONO, textTransform: 'none', letterSpacing: 0, fontWeight: 400 }}>
              {job.log.path}{job.log.modTime ? ` · written ${ago(job.log.modTime)}` : ''}
            </span>
          </div>
          <pre style={{
            margin: 0, background: 'var(--bg)', border: '1px solid var(--border)', borderRadius: '0.5rem',
            padding: '0.75rem 1rem', fontSize: 'var(--text-sm)', lineHeight: 1.55, fontFamily: MONO, color: 'var(--fg2)',
            maxHeight: '20rem', overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-word',
          }}>
            {job.log.error ? <span style={{ color: 'var(--yellow)' }}>{job.log.error}</span>
              : job.log.tail.length === 0 ? <span style={{ color: 'var(--fg3)' }}>(empty)</span>
              : job.log.tail.map((l, i) => {
                const trimmed = l.length > 300 ? l.slice(0, 300) + '…' : l;
                const color = errSet.has(trimmed) || errSet.has(l) ? 'var(--red)' : warnSet.has(trimmed) || warnSet.has(l) ? 'var(--yellow)' : undefined;
                return <div key={i} style={{ color }}>{l || ' '}</div>;
              })}
          </pre>
          {job.errors?.length > 0 && !job.log.tail.some(l => errSet.has(l)) && (
            <div style={{ marginTop: '0.5rem', fontSize: 'var(--text-sm)', fontFamily: MONO, color: 'var(--red)' }}>
              {job.errors.map((e, i) => <div key={i}>{e}</div>)}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ---------- the tab ----------

export default function JobsView({ jobs }) {
  const { data, error, loading, reload } = jobs;
  const [open, setOpen] = useState({});
  const [showThirdParty, setShowThirdParty] = useState(false);
  const toggle = label => setOpen(o => ({ ...o, [label]: !o[label] }));

  if (!data) {
    return (
      <div style={{ padding: '2.5rem', maxWidth: '50rem' }}>
        <h2 style={{ fontSize: 'var(--text-xl)', fontWeight: 700, marginBottom: '0.625rem' }}>Jobs</h2>
        <p style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)', lineHeight: 1.6, marginBottom: '1rem' }}>
          {loading ? 'Loading jobs…' : `${error}. Run the check once, or install the 15-minute job with scripts/install-service.sh:`}
        </p>
        {!loading && (
          <pre style={{ ...card, padding: '1rem', fontSize: 'var(--text-sm)', fontFamily: MONO, color: 'var(--fg2)' }}>{'cd cli && go build -o basecamp . && ./basecamp jobs'}</pre>
        )}
      </div>
    );
  }

  const scheduled = data.jobs.filter(j => j.kind === 'calendar' || j.kind === 'interval');
  const services = data.jobs.filter(j => j.kind === 'service');
  const other = data.jobs.filter(j => !['calendar', 'interval', 'service'].includes(j.kind));
  const red = data.jobs.filter(j => j.red);
  const stale = Date.now() - new Date(data.generatedAt) > 60 * 60_000;
  const gaps = (data.findings || []).filter(f => f.level !== 'info');
  // Group /tmp notes so five services don't take ten lines
  const tmpLabels = [...new Set(gaps.filter(f => /wiped on reboot/.test(f.text)).map(f => f.label))];
  const otherGaps = gaps.filter(f => !/wiped on reboot/.test(f.text));

  return (
    <div className="slide-in" style={{ padding: '2rem 2.5rem 3.75rem' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1.5rem', gap: '1.25rem' }}>
        <div>
          <h2 style={{ fontSize: 'var(--text-xl)', fontWeight: 700, letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.375rem' }}>
            <span>⏱</span>Jobs
          </h2>
          <p style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)' }}>
            Scheduled LaunchAgents on {data.host}, checked every 15 minutes · last check {ago(data.generatedAt)}
            {stale && <span style={{ color: 'var(--red)' }}> (is com.basecamp-jobs running?)</span>}
          </p>
        </div>
        <button onClick={reload} style={{
          padding: '0.5rem 1rem', fontSize: 'var(--text-sm)', fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit',
          background: 'var(--card2)', color: 'var(--fg)', border: '1px solid var(--border2)', borderRadius: '0.5rem',
        }}>{loading ? '⟳ Loading…' : '↻ Reload'}</button>
      </div>

      <div style={{ ...card, display: 'flex', gap: '1.5rem', padding: '1rem 1.25rem', marginBottom: '1rem', fontSize: 'var(--text-sm)', alignItems: 'center', flexWrap: 'wrap' }}>
        <span style={{ fontWeight: 700, color: red.length ? 'var(--red)' : 'var(--green)' }}>
          ● {red.length ? `${red.length} need${red.length === 1 ? 's' : ''} attention` : 'All jobs fine'}
        </span>
        <span style={{ color: 'var(--border2)' }}>|</span>
        <span><strong>{scheduled.length}</strong> <span style={{ color: 'var(--fg3)' }}>scheduled</span></span>
        <span><strong>{services.filter(s => s.status === 'running').length}/{services.length}</strong> <span style={{ color: 'var(--fg3)' }}>services up</span></span>
        {data.offsite?.image && (
          <span><span style={{ color: 'var(--fg3)' }}>offsite </span><span style={{ fontFamily: MONO }}>{data.offsite.image}</span> <span style={{ color: 'var(--fg3)' }}>{when(data.offsite.at)}</span></span>
        )}
      </div>

      {(otherGaps.length > 0 || tmpLabels.length > 0) && (
        <div style={{ ...card, padding: '1rem 1.25rem', marginBottom: '1.5rem' }}>
          <div style={{ ...sectionTitle, marginBottom: '0.5rem' }}>Gaps</div>
          {otherGaps.map((f, i) => (
            <div key={i} style={{ fontSize: 'var(--text-base)', color: 'var(--fg2)', padding: '0.375rem 0', borderTop: i ? '1px solid var(--border)' : 'none' }}>
              <span style={{ color: f.level === 'error' ? 'var(--red)' : 'var(--yellow)' }}>⚠</span>{' '}
              <span style={{ fontFamily: MONO, fontSize: 'var(--text-sm)' }}>{f.label}</span>: {f.text}
            </div>
          ))}
          {tmpLabels.length > 0 && (
            <div style={{ fontSize: 'var(--text-base)', color: 'var(--fg2)', padding: '0.375rem 0', borderTop: otherGaps.length ? '1px solid var(--border)' : 'none' }}>
              <span style={{ color: 'var(--yellow)' }}>⚠</span> Logging to /tmp (lost on reboot):{' '}
              <span style={{ fontFamily: MONO, fontSize: 'var(--text-sm)' }}>{tmpLabels.join(', ')}</span>
            </div>
          )}
        </div>
      )}

      <NightTimeline jobs={scheduled} />

      <div style={{ ...card, overflow: 'hidden', marginBottom: '1.5rem' }}>
        <div style={{
          display: 'grid', gridTemplateColumns: '5.5rem minmax(10rem, 1fr) 8rem 9rem minmax(0, 2fr) 1rem',
          gap: '1rem', padding: '0.625rem 1.25rem', ...sectionTitle, color: 'var(--fg4)', borderBottom: '1px solid var(--border)',
        }}>
          <span>Status</span><span>Job</span><span>Schedule</span><span>Last run</span><span>Result</span><span />
        </div>
        {scheduled.map(j => <JobRow key={j.label} job={j} open={!!open[j.label]} onToggle={() => toggle(j.label)} />)}
      </div>

      {services.length > 0 && (
        <div style={{ marginBottom: '1.5rem' }}>
          <div style={{ ...sectionTitle, marginBottom: '0.625rem' }}>Always-on services</div>
          <div style={{ ...card, overflow: 'hidden' }}>
            {[...services, ...other].map(j => <JobRow key={j.label} job={j} open={!!open[j.label]} onToggle={() => toggle(j.label)} />)}
          </div>
        </div>
      )}

      {data.thirdParty?.length > 0 && (
        <div style={{ marginBottom: '1.5rem' }}>
          <button onClick={() => setShowThirdParty(v => !v)} style={{
            ...sectionTitle, background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontFamily: 'inherit',
          }}>{showThirdParty ? '▾' : '▸'} Third-party agents ({data.thirdParty.length})</button>
          {showThirdParty && (
            <div style={{ ...card, padding: '0.625rem 1.25rem', marginTop: '0.625rem' }}>
              {data.thirdParty.map(t => (
                <div key={t.label} style={{ display: 'flex', gap: '1rem', fontSize: 'var(--text-sm)', padding: '0.375rem 0', color: 'var(--fg2)' }}>
                  <span style={{ fontFamily: MONO, flex: 1 }}>{t.label}</span>
                  <span style={{ color: 'var(--fg3)' }}>{t.kind}</span>
                  <span style={{ color: t.loaded ? 'var(--green)' : 'var(--fg4)', width: '5.5rem' }}>{t.loaded ? 'loaded' : 'not loaded'}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <p style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)', lineHeight: 1.6, maxWidth: '59rem' }}>
        A job can say what it did by writing <code style={{ fontFamily: MONO }}>{'~/Library/Application Support/jobs/<label>.json'}</code>{' '}
        with <code style={{ fontFamily: MONO }}>ok</code>, <code style={{ fontFamily: MONO }}>started</code>, <code style={{ fontFamily: MONO }}>finished</code> and{' '}
        <code style={{ fontFamily: MONO }}>summary</code> (see the README).
        {data.wantsStatusFile?.length > 0 && <> None of these do yet: <span style={{ fontFamily: MONO }}>{data.wantsStatusFile.join(', ')}</span>.</>}
      </p>
    </div>
  );
}
