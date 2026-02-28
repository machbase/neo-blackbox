import type { SettingsTab } from '../types/settings';

type SidebarProps = {
  activeTab: SettingsTab;
  onTabChange: (tab: SettingsTab) => void;
};

const tabItems: Array<{ id: SettingsTab; label: string }> = [
  { id: 'general', label: 'General' },
  { id: 'ffmpeg', label: 'FFmpeg Default' },
  { id: 'log', label: 'Log Configuration' },
];

export function Sidebar({ activeTab, onTabChange }: SidebarProps) {
  return (
    <aside className="sidebar-panel" aria-label="Server settings navigation">
      <div className="brand-block">
        <div className="brand-icon" aria-hidden="true">
          <span />
          <span />
          <span />
          <span />
        </div>
        <div>
          <p className="brand-title">Blackbox Server Admin</p>
          <p className="brand-subtitle">Configuration</p>
        </div>
      </div>

      <nav className="sidebar-nav" role="tablist" aria-label="Settings tabs">
        {tabItems.map((item) => {
          const isActive = item.id === activeTab;
          return (
            <button
              key={item.id}
              id={`tab-${item.id}`}
              className={`sidebar-tab ${isActive ? 'is-active' : ''}`}
              role="tab"
              type="button"
              aria-selected={isActive}
              aria-controls={`panel-${item.id}`}
              onClick={() => onTabChange(item.id)}
            >
              <span className="tab-dot" aria-hidden="true" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}
