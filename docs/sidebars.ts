import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

/**
 * Sidebar configuration for histui documentation.
 * Organized by outcome (what users want to accomplish) rather than just reference.
 */
const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    'installation',
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'quickstart/index',
        'quickstart/histui',
        'quickstart/histuid',
      ],
    },
    {
      type: 'category',
      label: 'Displaying Notifications',
      link: {
        type: 'doc',
        id: 'histuid/configuration',
      },
      items: [
        'histuid/integration',
        {
          type: 'category',
          label: 'Theming',
          link: {
            type: 'doc',
            id: 'histuid/theming/index',
          },
          items: [
            'histuid/theming/css-reference',
            'histuid/theming/css-inheritance',
            'histuid/theming/extending',
            'histuid/theming/layout-reference',
            'histuid/theming/manifest-reference',
            'histuid/theming/icon-aliases',
            'histuid/theming/animations',
            'histuid/theming/tinct-integration',
            'histuid/theming/advanced',
            'histuid/theming/examples',
          ],
        },
        'histuid/monitor-mode',
      ],
    },
    {
      type: 'category',
      label: 'Browsing History',
      items: [
        {
          type: 'category',
          label: 'Commands',
          link: {
            type: 'doc',
            id: 'histui/commands/index',
          },
          items: [
            'histui/commands/get',
            'histui/commands/tui',
            'histui/commands/status',
            'histui/commands/apps',
            'histui/commands/prune',
            'histui/commands/replay',
            'histui/commands/dnd',
            'histui/commands/set',
            'histui/commands/audio',
          ],
        },
        'histui/filtering',
      ],
    },
  ],
};

export default sidebars;
