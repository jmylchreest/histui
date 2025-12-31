import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'histui',
  tagline: 'A lightweight notification daemon and history browser for Wayland',
  favicon: 'img/favicon.ico',

  // GitHub Pages deployment
  url: 'https://jmylchreest.github.io',
  baseUrl: '/histui/',
  organizationName: 'jmylchreest',
  projectName: 'histui',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/jmylchreest/histui/tree/main/docs/',
          // Versioning configuration
          // lastVersion is set dynamically - see versions.json for available versions
          // The first entry in versions.json is the latest release
          includeCurrentVersion: true,
          versions: {
            current: {
              label: 'main',
              path: 'next',
              banner: 'unreleased',
            },
            // Released versions are configured dynamically via onBrokenLinks
            // The workflow updates lastVersion when creating new releases
          },
          // Default to latest released version (first in versions.json)
          lastVersion: require('./versions.json')[0] || 'current',
        },
        blog: false, // Disable blog
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  // Local search plugin (no external service dependency)
  plugins: [
    [
      '@cmfcmf/docusaurus-search-local',
      {
        indexDocs: true,
        indexBlog: false,
        indexPages: true,
        language: 'en',
        maxSearchResults: 8,
      },
    ],
  ],

  themeConfig: {
    // Color mode configuration
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },

    // Navigation bar
    navbar: {
      title: 'histui',
      logo: {
        alt: 'histui Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Documentation',
        },
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownActiveClassDisabled: true,
        },
        {
          href: 'https://github.com/jmylchreest/histui',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },

    // Footer
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/docs/quickstart'},
            {label: 'histui CLI', to: '/docs/histui/commands/get'},
            {label: 'histuid Daemon', to: '/docs/histuid/configuration'},
          ],
        },
        {
          title: 'Community',
          items: [
            {label: 'GitHub', href: 'https://github.com/jmylchreest/histui'},
            {label: 'Issues', href: 'https://github.com/jmylchreest/histui/issues'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} histui. Built with Docusaurus.`,
    },

    // Prism syntax highlighting
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'toml', 'css', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
