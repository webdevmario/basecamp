export function generatePlaybook(data) {
  const lines = [
    '#!/bin/bash',
    `# basecamp playbook — ${data.meta?.hostname || 'unknown'}`,
    `# Generated ${new Date(data.meta?.lastScan || Date.now()).toLocaleDateString()}`,
    '',
    '# === Homebrew ===',
    'if ! command -v brew &>/dev/null; then',
    '  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"',
    'fi',
    '',
  ];

  const brew = data.categories?.find(c => c.id === 'brew');
  if (brew?.items) {
    const formulae = brew.items
      .filter(i => i.detail?.includes('formula') && i.status !== 'stale')
      .map(i => i.name);
    const casks = brew.items
      .filter(i => i.detail?.includes('cask') && i.status !== 'stale')
      .map(i => i.name.toLowerCase().replace(/ /g, '-'));

    if (formulae.length) {
      lines.push('brew install ' + formulae.join(' \\\n  '));
      lines.push('');
    }
    if (casks.length) {
      lines.push('brew install --cask ' + casks.join(' \\\n  '));
      lines.push('');
    }
  }

  const vscode = data.categories?.find(c => c.id === 'vscode');
  if (vscode?.items) {
    lines.push('# === VS Code Extensions ===');
    vscode.items
      .filter(i => i.status !== 'stale')
      .forEach(ext => {
        // detail contains the extension ID (publisher.name or publisher.name@version)
        const id = ext.detail?.split('@')[0] || ext.name;
        lines.push(`code --install-extension ${id}`);
      });
    lines.push('');
  }

  const macos = data.categories?.find(c => c.id === 'macos');
  if (macos?.items) {
    lines.push('# === macOS Defaults ===');
    macos.items.forEach(pref => {
      if (pref.systemNote && pref.systemNote.startsWith('defaults ')) {
        lines.push(pref.systemNote);
      }
    });
    lines.push('killall Dock Finder');
    lines.push('');
  }

  // Version managers
  const versions = data.categories?.find(c => c.id === 'versions');
  if (versions?.items) {
    const nvmItems = versions.items.filter(i => i.versionManager === 'nvm');
    if (nvmItems.length) {
      lines.push('# === Node.js via nvm ===');
      lines.push('if ! command -v nvm &>/dev/null; then');
      lines.push('  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash');
      lines.push('fi');
      nvmItems.forEach(item => {
        item.versions?.forEach(v => {
          lines.push(`nvm install ${v.version}`);
          if (v.globals?.length) {
            lines.push(`nvm use ${v.version}`);
            const pkgNames = v.globals.map(g => g.split('@')[0]);
            lines.push(`npm install -g ${pkgNames.join(' ')}`);
          }
        });
      });
      lines.push('');
    }

    const pyenvItems = versions.items.filter(i => i.versionManager === 'pyenv');
    if (pyenvItems.length) {
      lines.push('# === Python via pyenv ===');
      lines.push('if ! command -v pyenv &>/dev/null; then');
      lines.push('  brew install pyenv');
      lines.push('fi');
      pyenvItems.forEach(item => {
        item.versions?.forEach(v => {
          lines.push(`pyenv install ${v.version}`);
          if (v.globals?.length) {
            lines.push(`pyenv shell ${v.version}`);
            const pkgNames = v.globals.map(g => g.split('@')[0]);
            lines.push(`pip install ${pkgNames.join(' ')}`);
          }
        });
      });
      lines.push('');
    }
  }

  return lines.join('\n');
}
