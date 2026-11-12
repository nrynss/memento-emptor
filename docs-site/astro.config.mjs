import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// The site is a static build, separate from the ledger web app. It has no
// server and reads nothing at build time beyond the repository's own files.
export default defineConfig({
  // Served by GitHub Pages at https://nrynss.github.io/memento-emptor/.
  site: 'https://nrynss.github.io',
  base: '/memento-emptor',
  integrations: [
    starlight({
      title: 'Memento Emptor',
      description:
        'Builder documentation for Memento Emptor, middleware a merchant runs beside its PayPal app to give shopping agents memory of why purchases came back.',
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
        { label: 'Sponsors', slug: 'sponsors' },
        { label: 'Submission summary', slug: 'submission-summary' },
      ],
    }),
  ],
});
