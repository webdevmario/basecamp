import React, { useState, useRef, useEffect } from 'react';

export default function NoteModal({ item, categoryId, userNote, onClose, onSave }) {
  const [val, setVal] = useState(userNote || '');
  const ref = useRef(null);
  useEffect(() => { setTimeout(() => ref.current?.focus(), 50); }, []);

  if (!item) return null;

  return (
    <div onClick={onClose} style={{
      position: 'fixed', inset: 0, zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)',
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        background: '#16171b', border: '1px solid #2a2a32', borderRadius: '1rem',
        width: '36rem', maxWidth: '90vw', padding: '2.25rem', boxShadow: '0 24px 80px rgba(0,0,0,0.5)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1.5rem' }}>
          <div>
            <div style={{ fontSize: 'var(--text-lg)', fontWeight: 700, marginBottom: '0.375rem' }}>{item.name}</div>
            <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)' }}>{item.detail}</div>
          </div>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', color: 'var(--fg3)', fontSize: 'var(--text-xl)', cursor: 'pointer', padding: '0 0.375rem',
          }}>✕</button>
        </div>

        {item.systemNote && (
          <div style={{
            fontSize: 'var(--text-sm)', color: 'var(--fg2)', background: 'rgba(255,255,255,0.03)',
            border: '1px solid var(--border2)', borderRadius: '0.625rem', padding: '0.75rem 1rem', marginBottom: '1.25rem', lineHeight: 1.5,
          }}>
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', display: 'block', marginBottom: '0.375rem' }}>System</span>
            {item.systemNote}
          </div>
        )}

        <div style={{ marginBottom: '1.25rem' }}>
          <label style={{ fontSize: 'var(--text-xs)', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', display: 'block', marginBottom: '0.5rem' }}>
            Your Notes
          </label>
          <textarea ref={ref} value={val} onChange={e => setVal(e.target.value)}
            placeholder="Add your thoughts, reminders, alternatives to try…"
            style={{
              width: '100%', minHeight: '8rem', padding: '1rem', fontSize: 'var(--text-base)', lineHeight: 1.6,
              background: 'var(--bg)', color: 'var(--fg)', border: '1px solid var(--border2)',
              borderRadius: '0.625rem', outline: 'none', fontFamily: "'Outfit', sans-serif",
              resize: 'vertical', boxSizing: 'border-box',
            }} />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.625rem' }}>
          <button onClick={onClose} style={{
            padding: '0.625rem 1.25rem', fontSize: 'var(--text-base)', fontWeight: 500, background: 'transparent',
            color: 'var(--fg2)', border: '1px solid var(--border2)', borderRadius: '0.5rem', cursor: 'pointer', fontFamily: 'inherit',
          }}>Cancel</button>
          <button onClick={() => { onSave(val); onClose(); }} style={{
            padding: '0.625rem 1.25rem', fontSize: 'var(--text-base)', fontWeight: 600, background: 'var(--accent)',
            color: '#000', border: 'none', borderRadius: '0.5rem', cursor: 'pointer', fontFamily: 'inherit',
          }}>Save</button>
        </div>
      </div>
    </div>
  );
}
