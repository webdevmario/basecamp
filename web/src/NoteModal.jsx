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
        background: '#16171b', border: '1px solid #2a2a32', borderRadius: 14,
        width: 460, maxWidth: '90vw', padding: 28, boxShadow: '0 24px 80px rgba(0,0,0,0.5)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 700, marginBottom: 4 }}>{item.name}</div>
            <div style={{ fontSize: 12, color: 'var(--fg3)' }}>{item.detail}</div>
          </div>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', color: 'var(--fg3)', fontSize: 18, cursor: 'pointer', padding: '0 4px',
          }}>✕</button>
        </div>

        {item.systemNote && (
          <div style={{
            fontSize: 12, color: 'var(--fg2)', background: 'rgba(255,255,255,0.03)',
            border: '1px solid var(--border2)', borderRadius: 8, padding: '10px 14px', marginBottom: 16, lineHeight: 1.5,
          }}>
            <span style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', display: 'block', marginBottom: 4 }}>System</span>
            {item.systemNote}
          </div>
        )}

        <div style={{ marginBottom: 16 }}>
          <label style={{ fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--fg3)', display: 'block', marginBottom: 6 }}>
            Your Notes
          </label>
          <textarea ref={ref} value={val} onChange={e => setVal(e.target.value)}
            placeholder="Add your thoughts, reminders, alternatives to try…"
            style={{
              width: '100%', minHeight: 100, padding: 12, fontSize: 13, lineHeight: 1.6,
              background: 'var(--bg)', color: 'var(--fg)', border: '1px solid var(--border2)',
              borderRadius: 8, outline: 'none', fontFamily: "'Outfit', sans-serif",
              resize: 'vertical', boxSizing: 'border-box',
            }} />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button onClick={onClose} style={{
            padding: '8px 16px', fontSize: 13, fontWeight: 500, background: 'transparent',
            color: 'var(--fg2)', border: '1px solid var(--border2)', borderRadius: 7, cursor: 'pointer', fontFamily: 'inherit',
          }}>Cancel</button>
          <button onClick={() => { onSave(val); onClose(); }} style={{
            padding: '8px 16px', fontSize: 13, fontWeight: 600, background: 'var(--accent)',
            color: '#000', border: 'none', borderRadius: 7, cursor: 'pointer', fontFamily: 'inherit',
          }}>Save</button>
        </div>
      </div>
    </div>
  );
}
