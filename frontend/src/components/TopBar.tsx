type TopBarProps = {
  breadcrumb: string;
  onSave: () => void;
  isSaving: boolean;
  saveStatusMessage: string;
  saveStatus: 'idle' | 'saving' | 'success' | 'error';
};

export function TopBar({ breadcrumb, onSave, isSaving, saveStatusMessage, saveStatus }: TopBarProps) {
  const statusClass =
    saveStatus === 'success' ? 'topbar-save-status is-success' :
      saveStatus === 'error' ? 'topbar-save-status is-error' : 'topbar-save-status';

  return (
    <header className="topbar">
      <p className="topbar-breadcrumb">Settings &gt; {breadcrumb}</p>

      <div className="topbar-actions">
        <div className="topbar-save-group">
          {saveStatusMessage && <span className={statusClass}>{saveStatusMessage}</span>}
          <button type="button" className="btn btn-primary btn-topbar" onClick={onSave} disabled={isSaving}>
            {isSaving ? 'Saving...' : 'Save Settings'}
          </button>
        </div>
      </div>
    </header>
  );
}
