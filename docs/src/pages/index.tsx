import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';

import styles from './index.module.css';

function HeroSection() {
  return (
    <section className={styles.hero}>
      <div className={styles.heroInner}>
        <h1 className={styles.title}>
          hist<span className={styles.titleAccent}>ui</span>
        </h1>
        <p className={styles.pronunciation}>/his-too-ee/</p>
        <p className={styles.tagline}>
          A highly configurable notification daemon and history browser for Wayland.
          Capture, display, and query your desktop notifications with style.
        </p>

        <div className={styles.buttons}>
          <Link className={styles.primaryBtn} to="/docs">
            Get Started
          </Link>
          <Link className={styles.secondaryBtn} to="https://github.com/jmylchreest/histui">
            View on GitHub
          </Link>
        </div>

        <div className={styles.codePreview}>
          <div className={styles.codeHeader}>
            <span className={`${styles.codeDot} ${styles.codeDotRed}`}></span>
            <span className={`${styles.codeDot} ${styles.codeDotYellow}`}></span>
            <span className={`${styles.codeDot} ${styles.codeDotGreen}`}></span>
            <span className={styles.codeTitle}>terminal</span>
          </div>
          <div className={styles.codeContent}>
            <div className={styles.codeLine}>
              <span className={styles.codePrompt}>$</span>
              <span className={styles.codeCommand}>histui get --app discord --since 1h</span>
            </div>
            <div className={styles.codeOutput}>
              <div>New message from @alice</div>
              <div>Server update in #general</div>
              <div>Call started in Voice Channel</div>
            </div>
            <div className={styles.codeLine} style={{marginTop: '1rem'}}>
              <span className={styles.codePrompt}>$</span>
              <span className={styles.codeCommand}>histui status</span>
            </div>
            <div className={styles.codeOutput}>
              <div>12 notifications (3 unread)</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

type FeatureItem = {
  icon: string;
  title: string;
  description: string;
};

const features: FeatureItem[] = [
  {
    icon: '🎨',
    title: 'CSS Theming with Hot Reload',
    description: 'Full CSS styling with live reload as you edit. Change colors, layouts, fonts, and animations without restarting.',
  },
  {
    icon: '🔗',
    title: 'Clickable Links & Deep Links',
    description: 'URLs in notifications are clickable. Deep links take you straight to the source app. Pango markup supported.',
  },
  {
    icon: '🖼️',
    title: 'Rich Content Display',
    description: 'Image previews, progress bars, action buttons. App icon aliases with Nerd Font fallbacks for 350+ apps.',
  },
  {
    icon: '📡',
    title: 'Monitor Mode',
    description: 'Run alongside dunst or mako to capture history without displaying popups. Best of both worlds.',
  },
  {
    icon: '🔍',
    title: 'Powerful History Search',
    description: 'Query past notifications by app, urgency, or time. Output as JSON, dmenu, or plain text for scripting.',
  },
  {
    icon: '🔊',
    title: 'Audio Alerts & Stacking',
    description: 'Per-urgency sound effects with repeat options. Smart notification stacking with animated transitions.',
  },
];

function FeaturesSection() {
  return (
    <section className={styles.features}>
      <div className={styles.featuresInner}>
        <h2 className={styles.featuresTitle}>Built for Modern Wayland</h2>
        <div className={styles.featuresGrid}>
          {features.map((feature, idx) => (
            <div key={idx} className={styles.featureCard}>
              <div className={styles.featureIcon}>{feature.icon}</div>
              <h3 className={styles.featureTitle}>{feature.title}</h3>
              <p className={styles.featureDesc}>{feature.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function InstallSection() {
  return (
    <section className={styles.install}>
      <div className={styles.installInner}>
        <h2 className={styles.installTitle}>Quick Install (Arch Linux)</h2>
        <div className={styles.installCode}>
          <span>
            <span className={styles.installPrompt}>$ </span>
            yay -S histui-bin
          </span>
        </div>
        <p style={{marginTop: '1rem', color: '#666', fontSize: '0.875rem'}}>
          See <Link to="/docs/installation">Installation Guide</Link> for other methods
        </p>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Home"
      description={siteConfig.tagline}>
      <HeroSection />
      <FeaturesSection />
      <InstallSection />
    </Layout>
  );
}
