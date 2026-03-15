const STORAGE_KEY = 'basecamp-user-notes';

export function loadNotes() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

export function saveNotes(notes) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(notes));
  } catch {
    // localStorage full or unavailable
  }
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
