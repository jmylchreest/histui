import React, { useState, useEffect, useRef } from 'react';
import styles from './AnimationDemo.module.css';
// Import the auto-generated effects from GTK4 source
import './effects-generated.css';

// Animation definitions matching the GTK4 effects.css
// Organized by type (Text = text-shadow, Box = box-shadow)
const ANIMATIONS = {
  // === TEXT EFFECTS (use text-shadow) ===
  'Text: Pulse & Glow': [
    { name: 'pulse-glow', duration: '2s', type: 'text' },
    { name: 'pulse-glow-subtle', duration: '3s', type: 'text' },
    { name: 'pulse-glow-intense', duration: '2s', type: 'text' },
    { name: 'pulse-glow-fast', duration: '0.8s', type: 'text' },
    { name: 'pulse-glow-slow', duration: '4s', type: 'text' },
  ],
  'Text: Shimmer & Sparkle': [
    { name: 'shimmer', duration: '2s', type: 'text' },
    { name: 'shimmer-fast', duration: '0.8s', type: 'text' },
    { name: 'sparkle', duration: '2.5s', type: 'text' },
    { name: 'sparkle-intense', duration: '2s', type: 'text' },
  ],
  'Text: Shadow Effects': [
    { name: 'text-shadow-breathe', duration: '3s', type: 'text' },
    { name: 'text-shadow-float', duration: '2s', type: 'text' },
    { name: 'text-shadow-dramatic', duration: '2s', type: 'text' },
  ],
  'Text: Attention': [
    { name: 'text-heartbeat', duration: '1.5s', type: 'text' },
    { name: 'text-urgent', duration: '0.6s', type: 'text' },
  ],
  'Text: Color Variants': [
    { name: 'pulse-error', duration: '2s', type: 'text' },
    { name: 'pulse-warning', duration: '2s', type: 'text' },
    { name: 'pulse-success', duration: '2s', type: 'text' },
    { name: 'pulse-accent', duration: '2s', type: 'text' },
    { name: 'sparkle-error', duration: '2.5s', type: 'text' },
    { name: 'sparkle-accent', duration: '2.5s', type: 'text' },
  ],
  // === BOX EFFECTS (use box-shadow) ===
  'Box: Glow Effects': [
    { name: 'border-glow', duration: '2s', type: 'box' },
    { name: 'border-glow-intense', duration: '2s', type: 'box' },
    { name: 'ring-glow', duration: '1.5s', type: 'box' },
    { name: 'inner-glow', duration: '2s', type: 'box' },
    { name: 'box-sparkle', duration: '3s', type: 'box' },
  ],
  'Box: Shadow Effects': [
    { name: 'shadow-breathe', duration: '3s', type: 'box' },
    { name: 'shadow-breathe-intense', duration: '3s', type: 'box' },
    { name: 'shadow-float', duration: '2s', type: 'box' },
    { name: 'shadow-float-intense', duration: '2s', type: 'box' },
    { name: 'shadow-dramatic', duration: '2s', type: 'box' },
    { name: 'shadow-dramatic-intense', duration: '2s', type: 'box' },
    { name: 'shadow-drop', duration: '2s', type: 'box' },
  ],
  'Box: Attention': [
    { name: 'attention-pulse', duration: '1.5s', type: 'box' },
    { name: 'attention-flash', duration: '2s', type: 'box' },
    { name: 'heartbeat', duration: '1.5s', type: 'box' },
    { name: 'urgent', duration: '0.6s', type: 'box' },
  ],
  'Box: Border Effects': [
    { name: 'border-pulse', duration: '2s', type: 'box' },
    { name: 'border-color-pulse', duration: '2s', type: 'box' },
    { name: 'border-fade', duration: '3s', type: 'box' },
  ],
  'Box: Color Variants': [
    { name: 'border-glow-error', duration: '2s', type: 'box' },
    { name: 'border-glow-warning', duration: '2s', type: 'box' },
    { name: 'border-glow-success', duration: '2s', type: 'box' },
    { name: 'border-glow-accent', duration: '2s', type: 'box' },
  ],
};

const TARGETS = [
  { id: 'summary', label: 'Title', selector: '[data-anim-target="summary"]' },
  { id: 'body', label: 'Body', selector: '[data-anim-target="body"]' },
  { id: 'icon', label: 'Icon', selector: '[data-anim-target="icon"]' },
  { id: 'popup', label: 'Popup Container', selector: '[data-anim-target="popup"]' },
  { id: 'image', label: 'Image', selector: '[data-anim-target="image"]' },
  { id: 'progress', label: 'Progress Bar', selector: '[data-anim-target="progress"]' },
];

const URGENCY_LEVELS = [
  { id: 'normal', label: 'Normal' },
  { id: 'low', label: 'Low' },
  { id: 'critical', label: 'Critical' },
];

// Placeholder image as inline SVG data URL
const PLACEHOLDER_IMAGE = `data:image/svg+xml,${encodeURIComponent(`
<svg xmlns="http://www.w3.org/2000/svg" width="400" height="120" viewBox="0 0 400 120">
  <defs>
    <linearGradient id="grad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#2c3e50;stop-opacity:1" />
      <stop offset="50%" style="stop-color:#34495e;stop-opacity:1" />
      <stop offset="100%" style="stop-color:#2c3e50;stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect width="400" height="120" fill="url(#grad)"/>
  <text x="200" y="55" text-anchor="middle" fill="rgba(255,255,255,0.4)" font-family="system-ui, sans-serif" font-size="14">
    Notification Image
  </text>
  <text x="200" y="75" text-anchor="middle" fill="rgba(255,255,255,0.25)" font-family="system-ui, sans-serif" font-size="11">
    (e.g., screenshot, album art)
  </text>
</svg>
`)}`;

// Generate customization template for an animation
function getCustomizationTemplate(animName: string, animType: string): string {
  const isText = animType === 'text';
  const shadowProp = isText ? 'text-shadow' : 'box-shadow';

  // Determine if this is a glow effect (needs light colors) or shadow effect (needs dark)
  const isGlow = animName.includes('glow') || animName.includes('pulse') ||
                 animName.includes('shimmer') || animName.includes('sparkle');

  let base: string;
  let peak: string;

  if (isGlow) {
    // Glow effects: centered (0 0), light colors
    if (isText) {
      base = `0 0 8px rgba(255, 255, 255, 0.4),
            0 0 16px rgba(255, 255, 255, 0.2)`;
      peak = `0 0 12px rgba(255, 255, 255, 0.7),
            0 0 24px rgba(255, 255, 255, 0.4),
            0 0 36px rgba(255, 255, 255, 0.2)`;
    } else {
      base = `0 0 8px rgba(255, 255, 255, 0.3),
            0 0 16px rgba(255, 255, 255, 0.15)`;
      peak = `0 0 16px rgba(255, 255, 255, 0.5),
            0 0 32px rgba(255, 255, 255, 0.3),
            0 0 48px rgba(255, 255, 255, 0.15)`;
    }
  } else {
    // Shadow effects: offset down, dark colors
    if (isText) {
      base = `0 2px 2px rgba(0, 0, 0, 0.9),
            0 4px 4px rgba(0, 0, 0, 0.6)`;
      peak = `0 4px 3px rgba(0, 0, 0, 0.95),
            0 8px 8px rgba(0, 0, 0, 0.7),
            0 12px 12px rgba(0, 0, 0, 0.4)`;
    } else {
      base = `0 2px 4px rgba(0, 0, 0, 0.6),
            0 4px 8px rgba(0, 0, 0, 0.4)`;
      peak = `0 4px 6px rgba(0, 0, 0, 0.7),
            0 8px 12px rgba(0, 0, 0, 0.5),
            0 16px 24px rgba(0, 0, 0, 0.3)`;
    }
  }

  const effectType = isGlow ? 'glow (light color, centered)' : 'shadow (dark color, offset)';

  return `/* Customize "${animName}"
 * Effect type: ${effectType}
 * Adjust: blur (3rd num), opacity (last num)
 */
@keyframes ${animName} {
    0%, 100% {
        ${shadowProp}: ${base};
    }
    50% {
        ${shadowProp}: ${peak};
    }
}`;
}

export default function AnimationDemo(): JSX.Element {
  const [selectedAnimation, setSelectedAnimation] = useState('pulse-glow');
  const [selectedTarget, setSelectedTarget] = useState('summary');
  const [selectedUrgency, setSelectedUrgency] = useState('critical');
  const [customCSS, setCustomCSS] = useState('');
  const styleRef = useRef<HTMLStyleElement | null>(null);

  // Get animation details
  const getAnimationDetails = (name: string) => {
    for (const category of Object.values(ANIMATIONS)) {
      const anim = category.find(a => a.name === name);
      if (anim) return anim;
    }
    return { name, duration: '2s', type: 'text' };
  };

  // Generate and inject dynamic CSS for the animation
  useEffect(() => {
    if (!styleRef.current) {
      styleRef.current = document.createElement('style');
      styleRef.current.id = 'animation-demo-styles';
      document.head.appendChild(styleRef.current);
    }

    const anim = getAnimationDetails(selectedAnimation);
    const target = TARGETS.find(t => t.id === selectedTarget);

    // Build CSS that applies animation to the selected target
    const css = `
      ${target?.selector} {
        animation: ${anim.name} ${anim.duration} ease-in-out infinite !important;
      }
      ${customCSS}
    `;

    styleRef.current.textContent = css;

    return () => {
      // Cleanup on unmount
    };
  }, [selectedAnimation, selectedTarget, customCSS]);

  // Cleanup style element on unmount
  useEffect(() => {
    return () => {
      if (styleRef.current && styleRef.current.parentNode) {
        styleRef.current.parentNode.removeChild(styleRef.current);
        styleRef.current = null;
      }
    };
  }, []);

  const urgencyClass = selectedUrgency === 'low'
    ? styles.urgencyLow
    : selectedUrgency === 'critical'
    ? styles.urgencyCritical
    : '';

  // Get the GTK4 selector for display
  const getGtkSelector = (targetId: string) => {
    const selectorMap: Record<string, string> = {
      summary: '.notification-popup.urgency-critical .notification-summary',
      body: '.notification-body',
      icon: '.notification-icon',
      popup: '.notification-popup',
      image: '.notification-image',
      progress: '.notification-progress',
    };
    return selectorMap[targetId] || '.notification-popup';
  };

  const currentAnim = getAnimationDetails(selectedAnimation);

  return (
    <div className={styles.demoContainer}>
      {/* Controls Row */}
      <div className={styles.controlsSection}>
        <div className={styles.controlsRow}>
          <div className={styles.controlGroup}>
            <label className={styles.controlLabel}>Animation</label>
            <select
              className={styles.selectControl}
              value={selectedAnimation}
              onChange={(e) => setSelectedAnimation(e.target.value)}
            >
              {Object.entries(ANIMATIONS).map(([category, anims]) => (
                <optgroup key={category} label={category}>
                  {anims.map((anim) => (
                    <option key={anim.name} value={anim.name}>
                      {anim.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </div>

          <div className={styles.controlGroup}>
            <label className={styles.controlLabel}>Target</label>
            <select
              className={styles.selectControl}
              value={selectedTarget}
              onChange={(e) => setSelectedTarget(e.target.value)}
            >
              {TARGETS.map((target) => (
                <option key={target.id} value={target.id}>
                  {target.label}
                </option>
              ))}
            </select>
          </div>

          <div className={styles.controlGroup}>
            <label className={styles.controlLabel}>Urgency</label>
            <select
              className={styles.selectControl}
              value={selectedUrgency}
              onChange={(e) => setSelectedUrgency(e.target.value)}
            >
              {URGENCY_LEVELS.map((level) => (
                <option key={level.id} value={level.id}>
                  {level.label}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Main Content: Preview + CSS Editor side by side */}
      <div className={styles.mainContent}>
        {/* Preview */}
        <div className={styles.previewSection}>
          <div className={styles.previewBackground}>
            <div
              className={`${styles.notificationPopup} ${urgencyClass}`}
              data-anim-target="popup"
            >
              <button className={styles.notificationClose}>x</button>

              <div className={styles.notificationHeader}>
                <div
                  className={styles.notificationIcon}
                  data-anim-target="icon"
                >
                  <svg width="32" height="32" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M21 10.12h-6.78l2.74-2.82c-2.73-2.7-7.15-2.8-9.88-.1-2.73 2.71-2.73 7.08 0 9.79s7.15 2.71 9.88 0C18.32 15.65 19 14.08 19 12.1h2c0 1.98-.88 4.55-2.64 6.29-3.51 3.48-9.21 3.48-12.72 0-3.5-3.47-3.53-9.11-.02-12.58s9.14-3.47 12.65 0L21 3v7.12zM12.5 8v4.25l3.5 2.08-.72 1.21L11 13V8h1.5z"/>
                  </svg>
                </div>
                <div className={styles.notificationTitleSection}>
                  <h4
                    className={styles.notificationSummary}
                    data-anim-target="summary"
                  >
                    System Update Available
                  </h4>
                  <div className={styles.notificationAppname}>Software Center</div>
                  <div className={styles.notificationTimestamp}>2 minutes ago</div>
                </div>
              </div>

              <div
                className={styles.notificationBody}
                data-anim-target="body"
              >
                A new system update is available. Click to install security patches and performance improvements.
              </div>

              <div
                className={styles.notificationProgress}
                data-anim-target="progress"
              >
                <div className={styles.progressFill}></div>
              </div>

              <div
                className={styles.notificationImageContainer}
                data-anim-target="image"
              >
                <img
                  src={PLACEHOLDER_IMAGE}
                  alt="Notification preview"
                  className={styles.notificationImage}
                />
              </div>

              <div className={styles.notificationActions}>
                <button className={styles.notificationAction}>Install Now</button>
                <button className={styles.notificationAction}>Later</button>
              </div>
            </div>
          </div>
        </div>

        {/* CSS Output & Editor */}
        <div className={styles.cssSection}>
          {/* GTK4 CSS Output */}
          <div className={styles.cssBlock}>
            <div className={styles.cssBlockHeader}>
              <span className={styles.cssBlockTitle}>Copy to your theme</span>
              <span className={styles.cssBlockBadge}>GTK4 CSS</span>
            </div>
            <pre className={styles.cssCode}>
{`${getGtkSelector(selectedTarget)} {
    animation: ${selectedAnimation} ${currentAnim.duration} ease-in-out infinite;
}`}
            </pre>
          </div>

          {/* Custom CSS Editor */}
          <div className={styles.cssBlock}>
            <div className={styles.cssBlockHeader}>
              <span className={styles.cssBlockTitle}>Live CSS Editor</span>
              <span className={styles.cssBlockBadge}>Preview Only</span>
            </div>
            <div className={styles.cssBlockHint}>
              <span>Customize the animation and preview changes live.</span>
              <button
                className={styles.loadTemplateBtn}
                onClick={() => setCustomCSS(getCustomizationTemplate(selectedAnimation, currentAnim.type))}
                type="button"
              >
                Load Template
              </button>
            </div>
            <textarea
              className={styles.cssEditor}
              value={customCSS}
              onChange={(e) => setCustomCSS(e.target.value)}
              placeholder={getCustomizationTemplate(selectedAnimation, currentAnim.type)}
              spellCheck={false}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
