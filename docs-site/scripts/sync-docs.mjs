// Copies the two documents that must not be hand-maintained into the content
// collection, so the site and the repository cannot disagree. The copies are
// git-ignored build output; ../docs holds the only source.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, '..', '..');
const outDir = join(here, '..', 'src', 'content', 'docs');

const pages = [
  {
    source: join(repoRoot, 'docs', 'interface.md'),
    target: join(outDir, 'interface.md'),
    title: 'Interface reference',
    description:
      'The eleven MCP tools, the turn pattern, the precedence order, and the responses a shopping agent gets.',
  },
  {
    source: join(repoRoot, 'docs', 'architecture.md'),
    target: join(outDir, 'architecture.md'),
    title: 'Architecture',
    description:
      'How a payment event becomes evidence and then a belief, where each store lives, and what the layer depends on.',
  },
];

mkdirSync(outDir, { recursive: true });

for (const page of pages) {
  let body = readFileSync(page.source, 'utf8');

  // The frontmatter title is the page's H1 in Starlight, so drop the source
  // document's own H1 instead of rendering two.
  body = body.replace(/^# .+\r?\n/, '');

  const note = [
    ':::note[Generated page]',
    `This page is copied from \`${page.source.slice(repoRoot.length + 1)}\` each time the site builds. Edit that file, not this page.`,
    ':::',
    '',
  ].join('\n');

  const frontmatter = ['---', `title: ${JSON.stringify(page.title)}`, `description: ${JSON.stringify(page.description)}`, '---', ''].join('\n');

  writeFileSync(page.target, `${frontmatter}\n${note}${body}`);
  console.log(`synced ${page.target.slice(repoRoot.length + 1)}`);
}
