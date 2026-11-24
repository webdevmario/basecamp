import React, { useState, useRef, useEffect } from 'react';

export default function NoteModal({ item, categoryId, userNote, onClose, onSave }) {
  const [val, setVal] = useState(userNote || '');
  const ref = useRef(null);
  useEffect(() => { setTimeout(() => ref.current?.focus(), 50); }, []);

  if (!item) return null;

  return (
    <div className="modal-scrim" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ width: '36rem', padding: '2rem', overflow: 'auto' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1.5rem' }}>
          <div>
            <div style={{ fontSize: 'var(--text-lg)', fontWeight: 700, marginBottom: '0.375rem' }}>{item.name}</div>
            <div style={{ fontSize: 'var(--text-sm)', color: 'var(--fg3)' }}>{item.detail}</div>
          </div>
          <button className="icon-btn" onClick={onClose} aria-label="Close" style={{ color: 'var(--fg3)', fontSize: 'var(--text-xl)', margin: '-0.5rem -0.5rem 0 0' }}>✕</button>
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
          <button className="btn btn-quiet" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={() => { onSave(val); onClose(); }}>Save</button>
        </div>
      </div>
    </div>
  );
}
