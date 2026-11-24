import { useEffect, useRef, useState } from 'react';

// Notes used to live only in this browser's localStorage. They now live on the server
// (~/.basecamp/notes.json via /api/notes) so every device sees the same ones;
// localStorage is the fallback when no server answers (plain `vite` without basecamp serve).
const STORAGE_KEY = 'basecamp-user-notes';

function loadLocal() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function saveLocal(notes) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(notes));
  } catch {
    // localStorage full or unavailable
  }
}

function saveServer(notes) {
  return fetch('/api/notes', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(notes),
  }).then(r => { if (!r.ok) throw new Error(`notes not saved (${r.status})`); });
}

// useNotes returns [notes, setNotes] like useState, loading from and saving to the server.
export function useNotes() {
  const [notes, setNotes] = useState({});
  const mode = useRef(null); // null until loaded, then 'server' or 'local'

  useEffect(() => {
    const local = loadLocal();
    fetch('/api/notes', { cache: 'no-store' })
      .then(r => (r.ok ? r.json() : Promise.reject()))
      .then(async server => {
        // One-time move of this browser's old notes up to the server.
        const merged = { ...local, ...server };
        if (Object.keys(local).some(k => !(k in server))) await saveServer(merged);
        localStorage.removeItem(STORAGE_KEY);
        mode.current = 'server';
        setNotes(merged);
      })
      .catch(() => {
        mode.current = 'local';
        setNotes(local);
      });
  }, []);

  const update = (next) => {
    setNotes(next);
    if (mode.current === 'server') saveServer(next).catch(err => console.error(err));
    else if (mode.current === 'local') saveLocal(next);
  };

  return [notes, update];
}

// Key format: "categoryId::itemName"
export function noteKey(categoryId, itemName) {
  return `${categoryId}::${itemName}`;
}

export function getNote(notes, categoryId, itemName) {
  return notes[noteKey(categoryId, itemName)] || '';
}

export function setNote(notes, categoryId, itemName, value) {
  const key = noteKey(categoryId, itemName);
  const next = { ...notes };
  if (value.trim()) {
    next[key] = value;
  } else {
    delete next[key];
  }
  return next;
}
