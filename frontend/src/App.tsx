import { useMemo, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { TopBar } from './components/TopBar';
import { ffmpegDefaults, generalSettings, logSettings } from './data/mockSettings';
import { FFmpegTab } from './tabs/FFmpegTab';
import { GeneralTab } from './tabs/GeneralTab';
import { LogTab } from './tabs/LogTab';
import type { SettingsTab } from './types/settings';

type TabMeta = {
  heading: string;
  subheading: string;
  breadcrumb: string;
  topActionLabel: string;
};

const TAB_META: Record<SettingsTab, TabMeta> = {
  general: {
    heading: 'General Settings',
    subheading: 'Configure core server paths and third-party integrations.',
    breadcrumb: 'General Settings',
    topActionLabel: 'Save Changes',
  },
  ffmpeg: {
    heading: 'FFmpeg Default Settings',
    subheading: 'Configure advanced binary paths and probe arguments for optimized media processing.',
    breadcrumb: 'FFmpeg Default Settings',
    topActionLabel: 'Save Changes',
  },
  log: {
    heading: 'Log Configuration',
    subheading: 'Manage how the server generates, stores, and rotates system log files.',
    breadcrumb: 'Log Configuration',
    topActionLabel: 'Save Changes',
  },
};

function App() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');
  const meta = useMemo(() => TAB_META[activeTab], [activeTab]);

  return (
    <div className="settings-shell">
      <Sidebar activeTab={activeTab} onTabChange={setActiveTab} />

      <main className="content-shell">
        <TopBar breadcrumb={meta.breadcrumb} saveLabel={meta.topActionLabel} />

        <section className="content-area">
          <header className="content-header">
            <h1>{meta.heading}</h1>
            <p>{meta.subheading}</p>
          </header>

          {activeTab === 'general' && <GeneralTab settings={generalSettings} />}
          {activeTab === 'ffmpeg' && <FFmpegTab settings={ffmpegDefaults} />}
          {activeTab === 'log' && <LogTab settings={logSettings} />}
        </section>
      </main>
    </div>
  );
}

export default App;
