export function assessCategory(cat) {
  const items = cat.items || [];
  const total = items.length;
  const stale = items.filter(i => i.status === 'stale').length;
  const outdated = items.filter(i => i.status === 'outdated').length;
  const good = total - stale - outdated;
  const score = total > 0 ? Math.round((good / total) * 100) : 100;
  const insights = [];

  if (cat.id === 'dotfiles') {
    items.filter(i => i.status === 'stale').forEach(f =>
      insights.push({ type: 'cleanup', text: `${f.name} appears unused — ${f.systemNote || 'consider removing'}` }));
    const shellFiles = items.filter(i => i.name.match(/\.(bash|zsh|zprofile)/));
    if (shellFiles.length > 3)
      insights.push({ type: 'observe', text: `${shellFiles.length} shell config files detected — some may be redundant` });
  }

  if (cat.id === 'brew') {
    items.filter(i => i.status === 'outdated').forEach(p =>
      insights.push({ type: 'update', text: `${p.name} has a newer version available` }));
    items.filter(i => i.status === 'stale').forEach(p =>
      insights.push({ type: 'cleanup', text: `${p.name} — ${p.systemNote || 'hasn\'t been used recently'}` }));
  }

  if (cat.id === 'vscode') {
    items.filter(i => i.status === 'stale').forEach(e =>
      insights.push({ type: 'cleanup', text: `${e.name} — ${e.systemNote || 'may be removable'}` }));
  }

  if (cat.id === 'globals') {
    items.filter(i => i.status === 'stale').forEach(p =>
      insights.push({ type: 'cleanup', text: `${p.name} — ${p.systemNote || 'potentially replaceable'}` }));
  }

  if (cat.id === 'security') {
    items.filter(i => i.status === 'stale').forEach(k =>
      insights.push({ type: 'security', text: `${k.name} — ${k.systemNote}` }));
    if (items.some(i => i.name.toLowerCase().includes('rsa')))
      insights.push({ type: 'security', text: 'Consider migrating remaining RSA keys to ed25519' });
  }

  if (cat.id === 'versions')
    insights.push({ type: 'observe', text: `${items.length} runtimes across ${new Set(items.map(i => (i.detail || '').split(' via ')[1]).filter(Boolean)).size} version managers` });

  if (cat.id === 'fonts') {
    items.filter(i => i.status === 'stale').forEach(f =>
      insights.push({ type: 'cleanup', text: `${f.name} — ${f.systemNote || 'not referenced in active configs'}` }));
  }

  if (cat.id === 'macos')
    insights.push({ type: 'observe', text: 'These preferences reset on a new machine — captured in the playbook' });

  if (cat.id === 'services') {
    const r = items.filter(i => i.status === 'running').length;
    insights.push({ type: 'observe', text: `${r} services at startup — review if all are needed daily` });
  }

  return { score, stale, outdated, good, total, insights };
}

export function globalHealth(categories, healthMap) {
  const all = categories.flatMap(c => c.items || []);
  const good = all.filter(i => !['stale', 'outdated'].includes(i.status)).length;
  return all.length > 0 ? Math.round((good / all.length) * 100) : 100;
}

export function totalIssues(categories, healthMap) {
  return categories.reduce((a, c) => {
    const h = healthMap[c.id];
    return a + (h ? h.stale + h.outdated : 0);
  }, 0);
}
