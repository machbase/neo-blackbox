import { useEffect, useMemo, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { TopBar } from './components/TopBar';
import { getConfig, postConfig } from './services/configApi';
import { buildFallbackApiConfigData, fromApiToDraft, toPostPayload } from './services/configMapper';
import { FFmpegTab } from './tabs/FFmpegTab';
import { GeneralTab } from './tabs/GeneralTab';
import { LogTab } from './tabs/LogTab';
import type { ConfigShadow, SettingsDraft, SettingsTab } from './types/settings';

type TabMeta = {
  heading: string;
  subheading: string;
  breadcrumb: string;
};

const TAB_META: Record<SettingsTab, TabMeta> = {
  general: {
    heading: 'General Settings',
    subheading: 'Configure core server paths and third-party integrations.',
    breadcrumb: 'General Settings',
  },
  ffmpeg: {
    heading: 'FFmpeg Default Settings',
    subheading: 'Configure default probe arguments for optimized media processing.',
    breadcrumb: 'FFmpeg Default Settings',
  },
  log: {
    heading: 'Log Configuration',
    subheading: 'Manage how the server generates, stores, and rotates system log files.',
    breadcrumb: 'Log Configuration',
  },
};

type SaveState = 'idle' | 'saving' | 'success' | 'error';

function buildInitialState(): { draft: SettingsDraft; shadow: ConfigShadow } {
  return fromApiToDraft(buildFallbackApiConfigData());
}

const INITIAL_STATE = buildInitialState();

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

function App() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');
  const [draft, setDraft] = useState<SettingsDraft>(INITIAL_STATE.draft);
  const [shadow, setShadow] = useState<ConfigShadow>(INITIAL_STATE.shadow);
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveStatusMessage, setSaveStatusMessage] = useState('');
  const meta = useMemo(() => TAB_META[activeTab], [activeTab]);

  useEffect(() => {
    let cancelled = false;

    async function loadConfig() {
      try {
        const apiData = await getConfig();
        if (cancelled) {
          return;
        }
        const mapped = fromApiToDraft(apiData);
        setDraft(mapped.draft);
        setShadow(mapped.shadow);
        setSaveState('idle');
        setSaveStatusMessage('');
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSaveState('error');
        setSaveStatusMessage(`Failed to load config: ${errorMessage(error, 'unknown error')}. Using fallback values.`);
      }
    }

    void loadConfig();

    return () => {
      cancelled = true;
    };
  }, []);

  const resetSaveStatus = () => {
    if (saveState !== 'idle' || saveStatusMessage !== '') {
      setSaveState('idle');
      setSaveStatusMessage('');
    }
  };

  const handleGeneralChange = (nextGeneral: SettingsDraft['general']) => {
    resetSaveStatus();
    setDraft((prev) => ({ ...prev, general: nextGeneral }));
  };

  const handleFFmpegChange = (nextFFmpeg: SettingsDraft['ffmpeg']) => {
    resetSaveStatus();
    setDraft((prev) => ({ ...prev, ffmpeg: nextFFmpeg }));
  };

  const handleLogChange = (nextLog: SettingsDraft['log']) => {
    resetSaveStatus();
    setDraft((prev) => ({ ...prev, log: nextLog }));
  };

  const handleSave = async () => {
    if (saveState === 'saving') {
      return;
    }

    setSaveState('saving');
    setSaveStatusMessage('');

    try {
      const payload = toPostPayload(draft, shadow);
      const response = await postConfig(payload);
      setSaveState('success');
      setSaveStatusMessage(response.reason || 'Settings saved successfully.');
    } catch (error) {
      setSaveState('error');
      setSaveStatusMessage(`Failed to save settings: ${errorMessage(error, 'unknown error')}`);
    }
  };

  return (
    <div className="settings-shell">
      <Sidebar activeTab={activeTab} onTabChange={setActiveTab} />

      <main className="content-shell">
        <TopBar
          breadcrumb={meta.breadcrumb}
          onSave={handleSave}
          isSaving={saveState === 'saving'}
          saveStatus={saveState}
          saveStatusMessage={saveStatusMessage}
        />

        <section className="content-area">
          <header className="content-header">
            <h1>{meta.heading}</h1>
            <p>{meta.subheading}</p>
          </header>

          {activeTab === 'general' && <GeneralTab settings={draft.general} onChange={handleGeneralChange} />}
          {activeTab === 'ffmpeg' && <FFmpegTab settings={draft.ffmpeg} onChange={handleFFmpegChange} />}
          {activeTab === 'log' && <LogTab settings={draft.log} onChange={handleLogChange} />}
        </section>
      </main>
    </div>
  );
}

export default App;
