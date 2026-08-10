import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const siteRoot = resolve(scriptDirectory, '..');
const realisticSecretPatterns = [
  /\bsk-(?:proj-|ant-)?[A-Za-z0-9_-]{16,}\b/,
  /\b(?:github_pat_|ghp_|glpat-|xox[baprs]-)[A-Za-z0-9_-]{20,}\b/,
  /\bAKIA[A-Z0-9]{16}\b/,
  /\bBearer\s+[A-Za-z0-9][A-Za-z0-9._~+/=-]{15,}\b/,
];
const internalDomainPattern = /\b(?:[a-z0-9-]+\.)+(?:internal|intranet|corp|local)(?:\.[a-z0-9.-]+)?\b/gi;

async function markdownFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return markdownFiles(path);
    return /\.mdx?$/i.test(entry.name) ? [path] : [];
  }));
  return files.flat().sort();
}

async function forbiddenTerms(termsPath) {
  return (await readFile(termsPath, 'utf8'))
    .split(/\r?\n/)
    .map((term) => term.trim())
    .filter((term) => term.length > 0 && !term.startsWith('#'));
}

export async function scanForbiddenTerms({
  docsDirectory = join(siteRoot, 'docs'),
  termsPath = join(siteRoot, 'quality', 'forbidden-terms.txt'),
} = {}) {
  const terms = await forbiddenTerms(termsPath);
  const findings = [];

  for (const filePath of await markdownFiles(docsDirectory)) {
    const lines = (await readFile(filePath, 'utf8')).split(/\r?\n/);
    lines.forEach((line, index) => {
      const location = `${filePath}:${index + 1}`;
      for (const term of terms) {
        if (line.includes(term)) findings.push(`${location}: forbidden term ${JSON.stringify(term)}`);
      }
      for (const domain of line.matchAll(internalDomainPattern)) {
        findings.push(`${location}: internal domain ${JSON.stringify(domain[0])}`);
      }
      if (realisticSecretPatterns.some((pattern) => pattern.test(line))) {
        findings.push(`${location}: realistic secret`);
      }
    });
  }

  return findings;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const findings = await scanForbiddenTerms();
  if (findings.length > 0) {
    console.error(findings.join('\n'));
    process.exitCode = 1;
  } else {
    console.log(`Forbidden content check passed for ${relative(siteRoot, join(siteRoot, 'docs')) || 'docs'}.`);
  }
}
