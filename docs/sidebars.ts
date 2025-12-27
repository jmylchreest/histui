import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

/**
 * Sidebar configuration for histui documentation.
 * Organized by tool (histui CLI, histuid daemon) with hierarchical structure.
 */
const sidebars: SidebarsConfig = {
  docs: [
    'intro',
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
      label: 'histui CLI',
      items: [
        {
          type: 'category',
          label: 'Commands',
          items: [
            'histui/commands/get',
            'histui/commands/prune',
            'histui/commands/status',
            'histui/commands/tui',
          ],
        },
        'histui/filtering',
      ],
    },
    {
      type: 'category',
      label: 'histuid Daemon',
      items: [
        'histuid/configuration',
        {
          type: 'category',
          label: 'Theming',
          items: [
            'histuid/theming/index',
            'histuid/theming/css-reference',
            'histuid/theming/examples',
          ],
        },
        'histuid/monitor-mode',
      ],
    },
  ],
};

export default sidebars;
