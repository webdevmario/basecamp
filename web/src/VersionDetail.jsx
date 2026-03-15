import React, { useState } from 'react';

export default function VersionDetail({ item, onClose }) {
  const [activeIdx, setActiveIdx] = useState(0);

  if (!item?.versions) return null;

  const pkgName = (s) => s.split('@')[0];
  const pkgVer = (s) => s.split('@')[1] || '';

  const primary = item.versions[0];
  const selected = item.versions[activeIdx];
  const isCurrent = activeIdx === 0;

  const primaryPkgs = {};
  primary.globals.forEach(g => { primaryPkgs[pkgName(g)] = pkgVer(g); });

  const selectedPkgs = {};
  selected.globals.forEach(g => { selectedPkgs[pkgName(g)] = pkgVer(g); });

  let rows = [];
  if (isCurrent) {
    rows = primary.globals.map(g => ({
      name: pkgName(g), version: pkgVer(g), status: 'installed',
    })).sort((a, b) => a.name.localeCompare(b.name));
  } else {
    const allNames = [...new Set([...Object.keys(primaryPkgs), ...Object.keys(selectedPkgs)])].sort();
    rows = allNames.map(name => {
      const inCurrent = primaryPkgs[name] || null;
      const inSelected = selectedPkgs[name] || null;
      let status = 'installed';
      if (inCurrent && !inSelected) status = 'missing';
      else if (!inCurrent && inSelected) status = 'extra';
      else if (inCurrent && inSelected && inCurrent !== inSelected) status = 'differs';
      return { name, version: inSelected, currentVersion: inCurrent, status };
    });
  }

  const missingPkgs = rows.filter(r => r.status === 'missing');

  let syncCmd = null;
  if (missingPkgs.length > 0 && !isCurrent) {
    if (item.versionManager === 'nvm') {
      syncCmd = `nvm use ${selected.version} && npm install -g ${missingPkgs.map(p => p.name).join(' ')}`;
    } else if (item.versionManager === 'pyenv') {
      syncCmd = `pyenv shell ${selected.version} && pip install ${missingPkgs.map(p => p.name).join(' ')}`;
    }
  }

  const statusColor = { installed: 'var(--green)', missing: 'var(--red)', extra: 'var(--accent)', differs: 'var(--yellow)' };
  const statusLabel = { installed: 'Installed', missing: 'Missing', extra: 'Extra', differs: 'Different' };

  return (
    <div onClick={onClose} style={{
      position: 'fixed', inset: 0, zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)',
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        background: '#16171b', border: '1px solid #2a2a32', borderRadius: 14,
        width: 520, maxWidth: '92vw', maxHeight: '85vh', display: 'flex', flexDirection: 'column',
        boxShadow: '0 24px 80px rgba(0,0,0,0.5)', overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ padding: '24px 24px 20px', flexShrink: 0 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 18 }}>
            <div>
              <div style={{ fontSize: 17, fontWeight: 700, marginBottom: 3 }}>{item.name}</div>
              <div style={{ fontSize: 12, color: 'var(--fg3)' }}>
                {item.versions.length} versions via {item.versionManager}
              </div>
            </div>
            <button onClick={onClose} style={{
              background: 'none', border: 'none', color: 'var(--fg3)', fontSize: 18, cursor: 'pointer', padding: '0 4px',
            }}>✕</button>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ fontSize: 12, color: 'var(--fg3)', fontWeight: 500 }}>Reviewing</span>
            <select
              value={activeIdx}
              onChange={e => setActiveIdx(parseInt(e.target.value))}
              style={{
                padding: '7px 32px 7px 12px', fontSize: 13, fontWeight: 600,
                fontFamily: "'JetBrains Mono', monospace",
                background: 'var(--card)', color: 'var(--fg)',
                border: '1px solid var(--border2)', borderRadius: 7,
                outline: 'none', cursor: 'pointer', appearance: 'none',
                backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%2371717a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E")`,
                backgroundRepeat: 'no-repeat', backgroundPosition: 'right 10px center',
              }}>
              {item.versions.map((v, i) => {
                let gapCount = 0;
                if (i > 0) {
                  const thisPkgs = new Set(v.globals.map(pkgName));
                  gapCount = primary.globals.filter(g => !thisPkgs.has(pkgName(g))).length;
                }
                const label = v.version + (v.label ? ` (${v.label})` : '') + (gapCount > 0 ? ` · ${gapCount} missing` : '');
                return <option key={v.version} value={i}>{label}</option>;
              })}
            </select>
            {!isCurrent && missingPkgs.length > 0 && (
              <span style={{
                fontSize: 11, fontWeight: 600, color: 'var(--red)',
                background: 'rgba(248,113,113,0.1)', padding: '4px 8px', borderRadius: 5,
              }}>{missingPkgs.length} gap{missingPkgs.length > 1 ? 's' : ''}</span>
            )}
          </div>
        </div>

        {/* Content */}
        <div style={{ flex: 1, overflow: 'auto', padding: '16px 24px 24px' }}>
          {!isCurrent && (
            <div style={{
              display: 'flex', gap: 16, fontSize: 12, marginBottom: 14, padding: '10px 14px',
              background: 'rgba(255,255,255,0.02)', borderRadius: 7, border: '1px solid var(--border)',
            }}>
              <span><span style={{ color: 'var(--green)' }}>●</span> <strong>{rows.filter(r => r.status === 'installed' || r.status === 'differs').length}</strong> <span style={{ color: 'var(--fg3)' }}>installed</span></span>
              {missingPkgs.length > 0 && <span><span style={{ color: 'var(--red)' }}>●</span> <strong>{missingPkgs.length}</strong> <span style={{ color: 'var(--fg3)' }}>missing vs current</span></span>}
            </div>
          )}

          <div style={{ background: 'var(--card)', borderRadius: 8, border: '1px solid var(--border)', overflow: 'hidden' }}>
            <div style={{
              display: 'grid', gridTemplateColumns: '1fr 90px 72px',
              gap: 8, padding: '7px 14px', fontSize: 10, fontWeight: 700,
              textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg4)',
              borderBottom: '1px solid var(--border)',
            }}>
              <span>Package</span>
              <span style={{ textAlign: 'center' }}>Version</span>
              <span style={{ textAlign: 'center' }}>{isCurrent ? '' : 'Status'}</span>
            </div>

            {rows.map((row, i) => (
              <div key={row.name} style={{
                display: 'grid', gridTemplateColumns: '1fr 90px 72px',
                gap: 8, padding: '8px 14px', alignItems: 'center',
                borderBottom: i < rows.length - 1 ? '1px solid var(--border)' : 'none',
                background: row.status === 'missing' ? 'rgba(248,113,113,0.03)' : 'transparent',
              }}>
                <div style={{
                  fontFamily: "'JetBrains Mono'", fontSize: 12, fontWeight: 500,
                  color: row.status === 'missing' ? 'var(--red)' : 'var(--fg)',
                }}>
                  {row.name}
                  {row.status === 'differs' && (
                    <span style={{ color: 'var(--fg3)', fontSize: 11, marginLeft: 6 }}>current: {row.currentVersion}</span>
                  )}
                </div>
                <div style={{ textAlign: 'center', fontFamily: "'JetBrains Mono'", fontSize: 11, color: row.status === 'missing' ? 'var(--fg4)' : 'var(--fg3)' }}>
                  {row.version || '—'}
                </div>
                <div style={{ display: 'flex', justifyContent: 'center' }}>
                  {!isCurrent && (
                    <span style={{
                      fontSize: 10, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase',
                      padding: '2px 0', borderRadius: 3, textAlign: 'center', width: 62,
                      display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                      color: statusColor[row.status], background: `${statusColor[row.status]}11`,
                    }}>{statusLabel[row.status]}</span>
                  )}
                </div>
              </div>
            ))}
          </div>

          {syncCmd && (
            <div style={{ marginTop: 16 }}>
              <div style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', marginBottom: 6 }}>
                Sync to match current
              </div>
              <div style={{
                background: 'var(--bg)', border: '1px solid var(--border2)', borderRadius: 7,
                padding: '10px 14px', fontFamily: "'JetBrains Mono'", fontSize: 11.5,
                color: 'var(--fg2)', lineHeight: 1.6, wordBreak: 'break-all',
              }}>{syncCmd}</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
