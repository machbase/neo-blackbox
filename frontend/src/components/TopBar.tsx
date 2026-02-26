type TopBarProps = {
  breadcrumb: string;
  saveLabel: string;
};

export function TopBar({ breadcrumb, saveLabel }: TopBarProps) {
  return (
    <header className="topbar">
      <p className="topbar-breadcrumb">Settings &gt; {breadcrumb}</p>

      <div className="topbar-actions">
        <label className="search-wrap" htmlFor="settings-search">
          <span className="search-icon" aria-hidden="true">⌕</span>
          <input id="settings-search" name="settings-search" type="text" placeholder="Search parameters..." />
        </label>

        <button type="button" className="btn btn-primary btn-topbar">
          {saveLabel}
        </button>
      </div>
    </header>
  );
}
