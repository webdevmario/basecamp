import React from 'react';

const STATUS_CONFIG = {
  active:  { color: 'var(--green)', bg: 'rgba(52,211,153,0.1)', label: 'OK' },
  current: { color: 'var(--green)', bg: 'rgba(52,211,153,0.1)', label: 'OK' },
  running: { color: 'var(--green)', bg: 'rgba(52,211,153,0.1)', label: 'Running' },
  outdated:{ color: 'var(--yellow)', bg: 'rgba(251,191,36,0.1)', label: 'Update' },
  stale:   { color: 'var(--red)', bg: 'rgba(248,113,113,0.1)', label: 'Review' },
};

export function getStatusConfig(status) {
  return STATUS_CONFIG[status] || STATUS_CONFIG.active;
}

export function ScoreRing({ score, size = 52 }) {
  const r = (size - 7) / 2;
  const c = 2 * Math.PI * r;
  const off = c - (score / 100) * c;
  const col = score >= 90 ? 'var(--green)' : score >= 70 ? 'var(--yellow)' : 'var(--red)';
  return (
    <svg width={size} height={size} style={{ transform: 'rotate(-90deg)', flexShrink: 0 }}>
      <circle cx={size/2} cy={size/2} r={r} fill="none" stroke="rgba(255,255,255,0.05)" strokeWidth={4.5} />
      <circle cx={size/2} cy={size/2} r={r} fill="none" stroke={col} strokeWidth={4.5}
        strokeDasharray={c} strokeDashoffset={off} strokeLinecap="round"
        style={{ transition: 'stroke-dashoffset 0.5s ease' }} />
      <text x={size/2} y={size/2} textAnchor="middle" dominantBaseline="central"
        fill={col} fontSize={size * 0.26} fontWeight="700" fontFamily="'Outfit', sans-serif"
        style={{ transform: 'rotate(90deg)', transformOrigin: 'center' }}>
        {score}
      </text>
    </svg>
  );
}

export function StatusBadge({ status }) {
  const cfg = getStatusConfig(status);
  return (
    <span style={{
      fontSize: 10, fontWeight: 700, letterSpacing: '0.05em', textTransform: 'uppercase',
      padding: '3px 0', borderRadius: 4, color: cfg.color, background: cfg.bg,
      textAlign: 'center', width: 62, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    }}>{cfg.label}</span>
  );
}

export const INSIGHT_ICONS = {
  cleanup: '🧹', update: '⬆️', observe: '👁', security: '🛡',
};
