import React, { useState, useMemo, useEffect } from 'react';
import { ScoreRing, StatusBadge, INSIGHT_ICONS } from './components';
import { assessCategory, globalHealth, totalIssues } from './health';
import { generatePlaybook } from './playbook';
import { loadNotes, saveNotes, getNote, setNote } from './notes';
import NoteModal from './NoteModal';
import VersionDetail from './VersionDetail';

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
  const [notes, setNotes] = useState(loadNotes());

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

  // Persist notes
  useEffect(() => { saveNotes(notes); }, [notes]);

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

  const simulateScan = () => {
    setScanning(true);
    // In production, this would call the CLI and reload
    // For now, just re-fetch
    fetch('/scan.json')
      .then(r => r.ok ? r.json() : Promise.reject())
      .then(d => { setData(d); setScanning(false); })
      .catch(() => setScanning(false));
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 24, marginBottom: 8 }}>▲</div>
          <div style={{ fontSize: 14, color: 'var(--fg3)' }}>Loading scan data…</div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      {/* Sidebar */}
      <div style={{
        width: 232, minWidth: 232, background: 'var(--card)',
        borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column',
      }}>
        <div style={{ padding: '22px 18px 18px', borderBottom: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, marginBottom: 10 }}>
            <div style={{
              width: 26, height: 26, borderRadius: 6,
              background: 'linear-gradient(135deg, var(--accent), #6366f1)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 12, fontWeight: 800, color: '#fff',
            }}>▲</div>
            <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: '-0.03em' }}>basecamp</span>
          </div>
          <div style={{ fontSize: 11, color: 'var(--fg3)', lineHeight: 1.5 }}>
            {data.meta.hostname}<br/>{data.meta.os}
          </div>
        </div>

        <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 12 }}>
          <ScoreRing score={gHealth} size={46} />
          <div>
            <div style={{ fontSize: 12, fontWeight: 600 }}>Health</div>
            <div style={{ fontSize: 11, color: 'var(--fg3)', marginTop: 1 }}>
              {tIssues === 0 ? 'All clear' : `${tIssues} to review`}
            </div>
          </div>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '8px 6px' }}>
          {data.categories.map(c => {
            const h = healthMap[c.id];
            const isActive = activeCat === c.id && !showPlaybook;
            const isHov = hoveredNav === c.id;
            const issues = h ? h.stale + h.outdated : 0;
            return (
              <button key={c.id}
                onMouseEnter={() => setHoveredNav(c.id)}
                onMouseLeave={() => setHoveredNav(null)}
                onClick={() => { setActiveCat(c.id); setShowPlaybook(false); setFilterIssues(false); }}
                style={{
                  display: 'flex', alignItems: 'center', gap: 9, width: '100%',
                  padding: '8px 10px', borderRadius: 7, border: 'none', cursor: 'pointer',
                  background: isActive ? 'var(--accent-bg)' : isHov ? 'var(--accent-hover)' : 'transparent',
                  color: isActive ? 'var(--accent)' : isHov ? 'var(--fg)' : 'var(--fg2)',
                  fontSize: 13, fontWeight: isActive ? 600 : 400,
                  transition: 'all 0.12s', textAlign: 'left', fontFamily: 'inherit', marginBottom: 1,
                }}>
                <span style={{ fontSize: 14, width: 20, textAlign: 'center' }}>{c.icon}</span>
                <span style={{ flex: 1 }}>{c.label}</span>
                {issues > 0 && (
                  <span style={{
                    fontSize: 10, fontWeight: 700, minWidth: 18, height: 18,
                    display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 9,
                    background: issues > 2 ? 'rgba(248,113,113,0.12)' : 'rgba(251,191,36,0.12)',
                    color: issues > 2 ? 'var(--red)' : 'var(--yellow)',
                  }}>{issues}</span>
                )}
              </button>
            );
          })}
        </div>

        <div style={{ padding: '10px 10px 14px', borderTop: '1px solid var(--border)', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div style={{ fontSize: 10, color: 'var(--fg4)', textAlign: 'center', marginBottom: 2 }}>
            Last scan: {new Date(data.meta.lastScan).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
          </div>
          <button onClick={simulateScan} disabled={scanning} style={{
            width: '100%', padding: '9px 0', borderRadius: 7, border: '1px solid var(--border2)',
            background: scanning ? 'var(--accent-bg)' : 'var(--card2)',
            color: scanning ? 'var(--accent)' : 'var(--fg)',
            fontSize: 12, fontWeight: 600, cursor: scanning ? 'default' : 'pointer',
            fontFamily: 'inherit', transition: 'all 0.15s', opacity: scanning ? 0.8 : 1,
          }}>
            {scanning ? '⟳ Scanning…' : '↻ Run Scan'}
          </button>
          <button onClick={() => setShowPlaybook(!showPlaybook)} style={{
            width: '100%', padding: '9px 0', borderRadius: 7,
            border: showPlaybook ? 'none' : '1px solid var(--border2)',
            background: showPlaybook ? 'var(--accent)' : 'transparent',
            color: showPlaybook ? '#000' : 'var(--fg3)',
            fontSize: 12, fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit', transition: 'all 0.15s',
          }}>
            {showPlaybook ? '✕ Close Playbook' : 'Export Playbook'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {error && !data.categories.length ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
            <div style={{ textAlign: 'center', maxWidth: 400, padding: 32 }}>
              <div style={{ fontSize: 40, marginBottom: 16 }}>▲</div>
              <h2 style={{ fontSize: 18, fontWeight: 700, marginBottom: 8 }}>No scan data yet</h2>
              <p style={{ fontSize: 13, color: 'var(--fg3)', lineHeight: 1.6, marginBottom: 20 }}>
                Run the CLI scanner to generate your environment snapshot:
              </p>
              <pre style={{
                background: 'var(--card)', border: '1px solid var(--border)', borderRadius: 8,
                padding: 16, fontSize: 12, color: 'var(--fg2)', fontFamily: "'JetBrains Mono'",
                textAlign: 'left',
              }}>
{`cd cli
go build -o basecamp .
./basecamp scan --pretty > ../web/public/scan.json`}
              </pre>
            </div>
          </div>
        ) : showPlaybook ? (
          <div className="slide-in" style={{ padding: 32, maxWidth: 780 }}>
            <h2 style={{ fontSize: 18, fontWeight: 700, marginBottom: 4 }}>Setup Playbook</h2>
            <p style={{ fontSize: 13, color: 'var(--fg3)', marginBottom: 20 }}>Generated from inventory · stale items excluded</p>
            <pre style={{
              background: 'var(--card)', border: '1px solid var(--border)', borderRadius: 10,
              padding: 20, fontSize: 12, lineHeight: 1.7, color: 'var(--fg2)', overflow: 'auto',
              fontFamily: "'JetBrains Mono', monospace", whiteSpace: 'pre-wrap', wordBreak: 'break-all',
            }}>{playbook}</pre>
          </div>
        ) : cat ? (
          <div className="slide-in" key={activeCat}>
            <div style={{ padding: '24px 32px 0' }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 18 }}>
                <div>
                  <h2 style={{ fontSize: 19, fontWeight: 700, letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: 9, marginBottom: 4 }}>
                    <span>{cat.icon}</span>{cat.label}
                  </h2>
                  <p style={{ fontSize: 13, color: 'var(--fg3)' }}>{cat.desc}</p>
                </div>
                <ScoreRing score={catHealth.score} size={52} />
              </div>

              {/* Summary strip */}
              <div style={{
                display: 'flex', gap: 20, padding: '12px 16px',
                background: 'var(--card)', borderRadius: 9, border: '1px solid var(--border)', marginBottom: 8,
                fontSize: 12, alignItems: 'center',
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
                  background: 'var(--card)', borderRadius: 9, border: '1px solid var(--border)',
                  padding: '14px 16px', marginBottom: 18,
                }}>
                  <div style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', marginBottom: 8 }}>Observations</div>
                  {catHealth.insights.map((ins, i) => (
                    <div key={i} style={{
                      display: 'flex', gap: 8, alignItems: 'flex-start',
                      fontSize: 12.5, color: 'var(--fg2)', lineHeight: 1.5, padding: '5px 0',
                      borderTop: i > 0 ? '1px solid var(--border)' : 'none',
                    }}>
                      <span style={{ flexShrink: 0, fontSize: 12 }}>{INSIGHT_ICONS[ins.type] || '💡'}</span>
                      <span>{ins.text}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Filter */}
              {(catHealth.stale + catHealth.outdated) > 0 && (
                <div style={{ marginBottom: 14 }}>
                  <button onClick={() => setFilterIssues(!filterIssues)} style={{
                    padding: '7px 12px', fontSize: 12, fontWeight: 600, cursor: 'pointer',
                    background: filterIssues ? 'rgba(248,113,113,0.08)' : 'var(--card)',
                    color: filterIssues ? 'var(--red)' : 'var(--fg3)',
                    border: `1px solid ${filterIssues ? 'rgba(248,113,113,0.2)' : 'var(--border)'}`,
                    borderRadius: 7, fontFamily: 'inherit', transition: 'all 0.15s',
                  }}>
                    {filterIssues ? '✕ Showing issues only' : `⚠ ${catHealth.stale + catHealth.outdated} items need attention`}
                  </button>
                </div>
              )}
            </div>

            {/* Item rows */}
            <div style={{ padding: '0 32px 48px' }}>
              <div style={{ background: 'var(--card)', borderRadius: 9, border: '1px solid var(--border)', overflow: 'hidden' }}>
                <div style={{
                  display: 'grid', gridTemplateColumns: 'minmax(130px, 1fr) 72px minmax(180px, 2fr) 36px',
                  gap: 10, padding: '8px 16px', fontSize: 10, fontWeight: 700,
                  textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg4)',
                  borderBottom: '1px solid var(--border)',
                }}>
                  <span>Name</span>
                  <span style={{ textAlign: 'center' }}>Status</span>
                  <span>Info</span>
                  <span></span>
                </div>

                {filteredItems.length === 0 ? (
                  <div style={{ padding: 28, textAlign: 'center', color: 'var(--fg3)', fontSize: 13 }}>
                    {filterIssues ? 'No issues — looking good' : 'No items in this category'}
                  </div>
                ) : filteredItems.map((item, i) => {
                  const hasVersions = item.versions?.length > 1;
                  const userNote = getNote(notes, activeCat, item.name);
                  const hasUserNote = userNote.trim().length > 0;
                  return (
                    <div key={i} style={{
                      display: 'grid', gridTemplateColumns: 'minmax(130px, 1fr) 72px minmax(180px, 2fr) 36px',
                      gap: 10, padding: '10px 16px', alignItems: 'center',
                      borderBottom: i < filteredItems.length - 1 ? '1px solid var(--border)' : 'none',
                      transition: 'background 0.1s', cursor: hasVersions ? 'pointer' : 'default',
                    }}
                    onClick={() => { if (hasVersions) setVersionItem(item); }}
                    onMouseEnter={e => e.currentTarget.style.background = 'var(--accent-hover)'}
                    onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                      <div style={{ overflow: 'hidden' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontFamily: "'JetBrains Mono'", fontSize: 12.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.name}</span>
                          {hasVersions && (
                            <span style={{
                              fontSize: 9, fontWeight: 700, color: 'var(--accent)', background: 'var(--accent-bg)',
                              padding: '1px 5px', borderRadius: 3, flexShrink: 0,
                            }}>{item.versions.length}v</span>
                          )}
                        </div>
                        <div style={{ fontSize: 11, color: 'var(--fg3)', marginTop: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.detail}</div>
                      </div>

                      <div style={{ display: 'flex', justifyContent: 'center' }}>
                        <StatusBadge status={item.status} />
                      </div>

                      <div style={{ fontSize: 12, color: 'var(--fg2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', lineHeight: 1.4 }}>
                        {item.systemNote || '—'}
                      </div>

                      <div style={{ display: 'flex', justifyContent: 'center' }}>
                        <button
                          onClick={(e) => { e.stopPropagation(); setModalItem(item); }}
                          title={hasUserNote ? 'View your note' : 'Add a note'}
                          style={{
                            width: 26, height: 26, borderRadius: 6, border: 'none', cursor: 'pointer',
                            background: hasUserNote ? 'var(--accent-bg)' : 'transparent',
                            color: hasUserNote ? 'var(--accent)' : 'var(--fg4)',
                            fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'center',
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
