import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const skipped = new Set(['.git', 'node_modules', 'dist', '.run-plan']);
const linkPattern = /!?\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g;
const failures = [];

function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!skipped.has(entry.name)) walk(path.join(directory, entry.name));
      continue;
    }
    if (entry.name.endsWith('.md')) check(path.join(directory, entry.name));
  }
}

function check(file) {
  const content = fs.readFileSync(file, 'utf8');
  for (const match of content.matchAll(linkPattern)) {
    const link = match[1].replace(/^<|>$/g, '');
    if (!link || link.startsWith('#') || /^(https?:|mailto:|tel:)/.test(link)) continue;
    const target = link.split('#', 1)[0];
    const resolved = path.resolve(path.dirname(file), target);
    if (!fs.existsSync(resolved)) failures.push(`${path.relative(root, file)} -> ${link}`);
  }
}

walk(root);
if (failures.length) {
  console.error(`Broken Markdown links:\n${failures.join('\n')}`);
  process.exit(1);
}
console.log('Markdown local links are valid.');
