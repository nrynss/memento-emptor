import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// The site is a static build, separate from the ledger web app. It has no
// server and reads nothing at build time beyond the repository's own files.
export default defineConfig({
  integrations: [
    starlight({
      title: 'Memento Emptor',
      description:
        'Builder documentation for a memory layer that turns classified payment outcomes into earned beliefs for shopping agents.',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/nrynss/memento-emptor',
        },
      ],
      sidebar: [
        { label: 'What the layer is', slug: 'what-the-layer-is' },
        { label: 'Interface reference', slug: 'interface' },
        { label: 'Architecture', slug: 'architecture' },
        { label: 'The demo and evidence', slug: 'demo-and-evidence' },
        { label: 'The comparison', slug: 'comparison' },
        { label: 'Submission summary', slug: 'submission-summary' },
      ],
    }),
  ],
});
