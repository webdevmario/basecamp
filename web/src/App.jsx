import React, { useState, useMemo, useEffect } from 'react';
import { ScoreRing, StatusBadge, INSIGHT_ICONS, PlaybookActions } from './components';
import { assessCategory, globalHealth, totalIssues } from './health';
import { generatePlaybook } from './playbook';
import { useNotes, getNote, setNote } from './notes';
import NoteModal from './NoteModal';
import VersionDetail from './VersionDetail';
import ConfigsView from './ConfigsView';
import JobsView, { useJobs, JobsNavButton } from './JobsView';

// Fallback data for when no scan.json exists yet
const EMPTY_DATA = {
  meta: { hostname: 'No scan data', os: 'Run `basecamp scan` to get started', lastScan: new Date().toISOString() },
  categories: [],
};

export default function App() {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [activeCat, setActiveCat] = useState(null);
  const [hoveredNav, setHoveredNav] = useState(null);
  const [filterIssues, setFilterIssues] = useState(false);
  const [showPlaybook, setShowPlaybook] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [modalItem, setModalItem] = useState(null);
  const [versionItem, setVersionItem] = useState(null);
  const [notes, setNotes] = useNotes();
  const [showJobs, setShowJobs] = useState(() => window.location.hash === '#jobs');
  const jobs = useJobs();
  useEffect(() => {
    const onHash = () => setShowJobs(window.location.hash === '#jobs');
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  // Load scan data
  useEffect(() => {
    fetch('/scan.json')
      .then(r => {
        if (!r.ok) throw new Error('No scan data found');
        return r.json();
      })
      .then(d => {
        setData(d);
        if (d.categories?.length) setActiveCat(d.categories[0].id);
        setLoading(false);
      })
      .catch(err => {
        setData(EMPTY_DATA);
        setLoading(false);
        setError(err.message);
      });
  }, []);

  const cat = data?.categories?.find(c => c.id === activeCat);

  const healthMap = useMemo(() => {
    if (!data?.categories) return {};
    const m = {};
    data.categories.forEach(c => { m[c.id] = assessCategory(c); });
    return m;
  }, [data]);

  const gHealth = useMemo(() => globalHealth(data?.categories || [], healthMap), [data, healthMap]);
  const tIssues = useMemo(() => totalIssues(data?.categories || [], healthMap), [data, healthMap]);
  const catHealth = healthMap[activeCat] || { score: 100, insights: [], stale: 0, outdated: 0, total: 0 };

  const filteredItems = useMemo(() => {
    if (!cat?.items) return [];
    if (!filterIssues) return cat.items;
    return cat.items.filter(i => i.status === 'stale' || i.status === 'outdated');
  }, [cat, filterIssues]);

  const playbook = useMemo(() => data ? generatePlaybook(data) : '', [data]);

  const handleSaveNote = (value) => {
    if (!modalItem) return;
    const next = setNote(notes, activeCat, modalItem.name, value);
    setNotes(next);
  };

  // Asks basecamp serve to scan now; without a server, just reloads the last scan.
  const runScan = () => {
    setScanning(true);
    fetch('/api/scan', { method: 'POST' })
      .then(r => r.ok ? r : fetch('/scan.json', { cache: 'no-store' }))
      .then(r => r.ok ? r.json() : Promise.reject())
      .then(d => { setData(d); setScanning(false); })
      .catch(() => setScanning(false));
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 'var(--text-2xl)', marginBottom: '0.625rem' }}>▲</div>
          <div style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)' }}>Loading scan data…</div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      {/* Sidebar */}
      <div style={{
        width: '18rem', minWidth: '18rem', background: 'var(--card)',
        borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column',
      }}>
        <div style={{ padding: '1.75rem 1.5rem 1.5rem', borderBottom: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.75rem' }}>
            <div style={{
              width: '2rem', height: '2rem', borderRadius: '0.5rem',
              background: 'linear-gradient(135deg, var(--accent), #6366f1)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 'var(--text-sm)', fontWeight: 800, color: '#fff',
            }}>▲</div>
            <span style={{ fontSize: 'var(--text-lg)', fontWeight: 700, letterSpacing: '-0.03em' }}>basecamp</span>
          </div>
          <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)', lineHeight: 1.5 }}>
            {data.meta.hostname}<br/>{data.meta.os}
          </div>
        </div>

        <div style={{ padding: '1rem 1.25rem', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <ScoreRing score={gHealth} size={46} />
          <div>
            <div style={{ fontSize: 'var(--text-sm)', fontWeight: 600 }}>Health</div>
            <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)', marginTop: 1 }}>
              {tIssues === 0 ? 'All clear' : `${tIssues} to review`}
            </div>
          </div>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '0.625rem 0.5rem' }}>
          <JobsNavButton jobs={jobs} active={showJobs} onClick={() => { setShowJobs(true); setShowPlaybook(false); history.replaceState(null, '', '#jobs'); }} />
          {data.categories.map(c => {
            const h = healthMap[c.id];
            const isActive = activeCat === c.id && !showPlaybook && !showJobs;
            const isHov = hoveredNav === c.id;
            const issues = h ? h.stale + h.outdated : 0;
            return (
              <button key={c.id}
                onMouseEnter={() => setHoveredNav(c.id)}
                onMouseLeave={() => setHoveredNav(null)}
                onClick={() => { setActiveCat(c.id); setShowPlaybook(false); setShowJobs(false); setFilterIssues(false); history.replaceState(null, '', ' '); }}
                style={{
                  display: 'flex', alignItems: 'center', gap: '0.75rem', width: '100%',
                  padding: '0.625rem 0.75rem', borderRadius: '0.5rem', border: 'none', cursor: 'pointer',
                  background: isActive ? 'var(--accent-bg)' : isHov ? 'var(--accent-hover)' : 'transparent',
                  color: isActive ? 'var(--accent)' : isHov ? 'var(--fg)' : 'var(--fg2)',
                  fontSize: 'var(--text-base)', fontWeight: isActive ? 600 : 400,
                  transition: 'all 0.12s', textAlign: 'left', fontFamily: 'inherit', marginBottom: 1,
                }}>
                <span style={{ fontSize: 'var(--text-base)', width: '1.5rem', textAlign: 'center' }}>{c.icon}</span>
                <span style={{ flex: 1 }}>{c.label}</span>
                {issues > 0 && (
                  <span style={{
                    fontSize: 'var(--text-xs)', fontWeight: 700, minWidth: '1.5rem', height: '1.5rem',
                    display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: '0.75rem',
                    background: issues > 2 ? 'rgba(248,113,113,0.12)' : 'rgba(251,191,36,0.12)',
                    color: issues > 2 ? 'var(--red)' : 'var(--yellow)',
                  }}>{issues}</span>
                )}
              </button>
            );
          })}
          <button onClick={() => { setActiveCat('configs'); setShowPlaybook(false); setShowJobs(false); history.replaceState(null, '', ' '); }} style={{
            display: 'flex', alignItems: 'center', gap: '0.75rem', width: '100%', padding: '0.625rem 0.75rem', borderRadius: '0.5rem',
            border: 'none', cursor: 'pointer', textAlign: 'left', fontFamily: 'inherit', fontSize: 'var(--text-base)', marginTop: '0.5rem',
            background: activeCat === 'configs' && !showPlaybook && !showJobs ? 'var(--accent-bg)' : 'transparent',
            color: activeCat === 'configs' && !showPlaybook && !showJobs ? 'var(--accent)' : 'var(--fg2)',
            fontWeight: activeCat === 'configs' && !showPlaybook && !showJobs ? 600 : 400,
          }}>
            <span style={{ fontSize: 'var(--text-base)', width: '1.5rem', textAlign: 'center' }}>🗂</span>
            <span style={{ flex: 1 }}>Configs</span>
          </button>
        </div>

        <div style={{ padding: '0.75rem 0.75rem 1rem', borderTop: '1px solid var(--border)', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <div style={{ fontSize: 'var(--text-xs)', color: 'var(--fg4)', textAlign: 'center', marginBottom: 2 }}>
            Last scan: {new Date(data.meta.lastScan).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
          </div>
          <button onClick={runScan} disabled={scanning} style={{
            width: '100%', padding: '0.75rem 0', borderRadius: '0.5rem', border: '1px solid var(--border2)',
            background: scanning ? 'var(--accent-bg)' : 'var(--card2)',
            color: scanning ? 'var(--accent)' : 'var(--fg)',
            fontSize: 'var(--text-sm)', fontWeight: 600, cursor: scanning ? 'default' : 'pointer',
            fontFamily: 'inherit', transition: 'all 0.15s', opacity: scanning ? 0.8 : 1,
          }}>
            {scanning ? '⟳ Scanning…' : '↻ Run Scan'}
          </button>
          <button onClick={() => { setShowPlaybook(!showPlaybook); setShowJobs(false); }} style={{
            width: '100%', padding: '0.75rem 0', borderRadius: '0.5rem',
            border: showPlaybook ? 'none' : '1px solid var(--border2)',
            background: showPlaybook ? 'var(--accent)' : 'transparent',
            color: showPlaybook ? '#000' : 'var(--fg3)',
            fontSize: 'var(--text-sm)', fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit', transition: 'all 0.15s',
          }}>
            {showPlaybook ? '✕ Close Playbook' : 'Export Playbook'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {showJobs ? (
          <JobsView jobs={jobs} />
        ) : error && !data.categories.length ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
            <div style={{ textAlign: 'center', maxWidth: '31rem', padding: '2.5rem' }}>
              <div style={{ fontSize: '3.25rem', marginBottom: '1.25rem' }}>▲</div>
              <h2 style={{ fontSize: 'var(--text-xl)', fontWeight: 700, marginBottom: '0.625rem' }}>No scan data yet</h2>
              <p style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)', lineHeight: 1.6, marginBottom: '1.5rem' }}>
                Run the CLI scanner to generate your environment snapshot:
              </p>
              <pre style={{
                background: 'var(--card)', border: '1px solid var(--border)', borderRadius: '0.625rem',
                padding: '1.25rem', fontSize: 'var(--text-sm)', color: 'var(--fg2)', fontFamily: "'JetBrains Mono'",
                textAlign: 'left',
              }}>
{`cd cli
go build -o basecamp .
./basecamp scan -o ~/.basecamp/scan.json
./basecamp serve`}
              </pre>
            </div>
          </div>
        ) : showPlaybook ? (
          <div className="slide-in" style={{ padding: '2.5rem', maxWidth: '61rem' }}>
            <h2 style={{ fontSize: 'var(--text-xl)', fontWeight: 700, marginBottom: '0.375rem' }}>Setup Playbook</h2>
            <p style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)', marginBottom: '1rem' }}>Generated from inventory · stale items excluded</p>
            <PlaybookActions script={playbook} hostname={data.meta.hostname} />
            <pre style={{
              background: 'var(--card)', border: '1px solid var(--border)', borderRadius: '0.75rem',
              padding: '1.5rem', fontSize: 'var(--text-sm)', lineHeight: 1.7, color: 'var(--fg2)', overflow: 'auto',
              fontFamily: "'JetBrains Mono', monospace", whiteSpace: 'pre-wrap', wordBreak: 'break-all',
            }}>{playbook}</pre>
          </div>
        ) : activeCat === 'configs' ? (
          <ConfigsView />
        ) : cat ? (
          <div className="slide-in" key={activeCat}>
            <div style={{ padding: '2rem 2.5rem 0' }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <div>
                  <h2 style={{ fontSize: 'var(--text-xl)', fontWeight: 700, letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.375rem' }}>
                    <span>{cat.icon}</span>{cat.label}
                  </h2>
                  <p style={{ fontSize: 'var(--text-base)', color: 'var(--fg3)' }}>{cat.desc}</p>
                </div>
                <ScoreRing score={catHealth.score} size={52} />
              </div>

              {/* Summary strip */}
              <div style={{
                display: 'flex', gap: '1.5rem', padding: '1rem 1.25rem',
                background: 'var(--card)', borderRadius: '0.75rem', border: '1px solid var(--border)', marginBottom: '0.625rem',
                fontSize: 'var(--text-sm)', alignItems: 'center',
              }}>
                <span><span style={{ color: 'var(--fg3)' }}>Total </span><strong style={{ fontFamily: "'JetBrains Mono'" }}>{catHealth.total}</strong></span>
                <span style={{ color: 'var(--border)' }}>|</span>
                <span><span style={{ color: 'var(--green)' }}>●</span> <strong>{catHealth.total - catHealth.stale - catHealth.outdated}</strong> <span style={{ color: 'var(--fg3)' }}>good</span></span>
                {catHealth.outdated > 0 && <><span style={{ color: 'var(--border)' }}>|</span><span><span style={{ color: 'var(--yellow)' }}>●</span> <strong>{catHealth.outdated}</strong> <span style={{ color: 'var(--fg3)' }}>outdated</span></span></>}
                {catHealth.stale > 0 && <><span style={{ color: 'var(--border)' }}>|</span><span><span style={{ color: 'var(--red)' }}>●</span> <strong>{catHealth.stale}</strong> <span style={{ color: 'var(--fg3)' }}>to review</span></span></>}
              </div>

              {/* Insights */}
              {catHealth.insights.length > 0 && (
                <div style={{
                  background: 'var(--card)', borderRadius: '0.75rem', border: '1px solid var(--border)',
                  padding: '1rem 1.25rem', marginBottom: '1.5rem',
                }}>
                  <div style={{ fontSize: 'var(--text-xs)', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', marginBottom: '0.625rem' }}>Observations</div>
                  {catHealth.insights.map((ins, i) => (
                    <div key={i} style={{
                      display: 'flex', gap: '0.625rem', alignItems: 'flex-start',
                      fontSize: 'var(--text-base)', color: 'var(--fg2)', lineHeight: 1.5, padding: '0.375rem 0',
                      borderTop: i > 0 ? '1px solid var(--border)' : 'none',
                    }}>
                      <span style={{ flexShrink: 0, fontSize: 'var(--text-sm)' }}>{INSIGHT_ICONS[ins.type] || '💡'}</span>
                      <span>{ins.text}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Filter */}
              {(catHealth.stale + catHealth.outdated) > 0 && (
                <div style={{ marginBottom: '1rem' }}>
                  <button onClick={() => setFilterIssues(!filterIssues)} style={{
                    padding: '0.5rem 1rem', fontSize: 'var(--text-sm)', fontWeight: 600, cursor: 'pointer',
                    background: filterIssues ? 'rgba(248,113,113,0.08)' : 'var(--card)',
                    color: filterIssues ? 'var(--red)' : 'var(--fg3)',
                    border: `1px solid ${filterIssues ? 'rgba(248,113,113,0.2)' : 'var(--border)'}`,
                    borderRadius: '0.5rem', fontFamily: 'inherit', transition: 'all 0.15s',
                  }}>
                    {filterIssues ? '✕ Showing issues only' : `⚠ ${catHealth.stale + catHealth.outdated} items need attention`}
                  </button>
                </div>
              )}
            </div>

            {/* Item rows */}
            <div style={{ padding: '0 2.5rem 3.75rem' }}>
              <div style={{ background: 'var(--card)', borderRadius: '0.75rem', border: '1px solid var(--border)', overflow: 'hidden' }}>
                <div style={{
                  display: 'grid', gridTemplateColumns: 'minmax(10rem, 1fr) 5.75rem minmax(14rem, 2fr) 2.75rem',
                  gap: '0.75rem', padding: '0.625rem 1.25rem', fontSize: 'var(--text-xs)', fontWeight: 700,
                  textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg4)',
                  borderBottom: '1px solid var(--border)',
                }}>
                  <span>Name</span>
                  <span style={{ textAlign: 'center' }}>Status</span>
                  <span>Info</span>
                  <span></span>
                </div>

                {filteredItems.length === 0 ? (
                  <div style={{ padding: '2.25rem', textAlign: 'center', color: 'var(--fg3)', fontSize: 'var(--text-base)' }}>
                    {filterIssues ? 'No issues, looking good' : 'No items in this category'}
                  </div>
                ) : filteredItems.map((item, i) => {
                  const hasVersions = item.versions?.length > 1;
                  const userNote = getNote(notes, activeCat, item.name);
                  const hasUserNote = userNote.trim().length > 0;
                  return (
                    <div key={i} style={{
                      display: 'grid', gridTemplateColumns: 'minmax(10rem, 1fr) 5.75rem minmax(14rem, 2fr) 2.75rem',
                      gap: '0.75rem', padding: '0.75rem 1.25rem', alignItems: 'center',
                      borderBottom: i < filteredItems.length - 1 ? '1px solid var(--border)' : 'none',
                      transition: 'background 0.1s', cursor: hasVersions ? 'pointer' : 'default',
                    }}
                    onClick={() => { if (hasVersions) setVersionItem(item); }}
                    onMouseEnter={e => e.currentTarget.style.background = 'var(--accent-hover)'}
                    onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                      <div style={{ overflow: 'hidden' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                          <span style={{ fontFamily: "'JetBrains Mono'", fontSize: 'var(--text-base)', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.name}</span>
                          {hasVersions && (
                            <span style={{
                              fontSize: 'var(--text-xs)', fontWeight: 700, color: 'var(--accent)', background: 'var(--accent-bg)',
                              padding: '1px 0.375rem', borderRadius: '0.25rem', flexShrink: 0,
                            }}>{item.versions.length}v</span>
                          )}
                        </div>
                        <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)', marginTop: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.detail}</div>
                      </div>

                      <div style={{ display: 'flex', justifyContent: 'center' }}>
                        <StatusBadge status={item.status} />
                      </div>

                      <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', lineHeight: 1.4 }}>
                        {item.systemNote || '-'}
                      </div>

                      <div style={{ display: 'flex', justifyContent: 'center' }}>
                        <button
                          onClick={(e) => { e.stopPropagation(); setModalItem(item); }}
                          title={hasUserNote ? 'View your note' : 'Add a note'}
                          style={{
                            width: '2rem', height: '2rem', borderRadius: '0.5rem', border: 'none', cursor: 'pointer',
                            background: hasUserNote ? 'var(--accent-bg)' : 'transparent',
                            color: hasUserNote ? 'var(--accent)' : 'var(--fg4)',
                            fontSize: 'var(--text-base)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                            transition: 'all 0.12s',
                          }}
                          onMouseEnter={e => { e.currentTarget.style.background = 'rgba(129,140,248,0.15)'; e.currentTarget.style.color = 'var(--accent)'; }}
                          onMouseLeave={e => { e.currentTarget.style.background = hasUserNote ? 'var(--accent-bg)' : 'transparent'; e.currentTarget.style.color = hasUserNote ? 'var(--accent)' : 'var(--fg4)'; }}
                        >
                          {hasUserNote ? '✎' : '＋'}
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        ) : null}
      </div>

      {/* Modals */}
      {modalItem && (
        <NoteModal
          item={modalItem}
          categoryId={activeCat}
          userNote={getNote(notes, activeCat, modalItem.name)}
          onClose={() => setModalItem(null)}
          onSave={handleSaveNote}
        />
      )}
      {versionItem && (
        <VersionDetail item={versionItem} onClose={() => setVersionItem(null)} />
      )}
    </div>
  );
}
