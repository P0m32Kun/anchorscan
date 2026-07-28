import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const skipped = new Set(['.git', 'node_modules', 'dist', '.run-plan']);
const linkPattern = /!?\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g;
const failures = [];
const staleContracts = [
  /双引擎规则表/,
  /dual-engine matrix/,
  /项目默认目标发起扫描/,
];

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
  for (const pattern of staleContracts) {
    if (pattern.test(content)) failures.push(`${path.relative(root, file)} contains stale contract: ${pattern}`);
  }
  if (file.includes(`${path.sep}docs${path.sep}adr${path.sep}`) && /docs\/plans\/(?:project-engagement-report|add-builtin-vulnerability-probes|harden-scan-confidence)\//.test(content)) {
    failures.push(`${path.relative(root, file)} references a non-current, non-archive plan path`);
  }
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
console.log('Markdown local links and focused documentation contracts are valid.');
