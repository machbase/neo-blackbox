import type { FFmpegDefaults } from '../types/settings';

type FFmpegTabProps = {
  settings: FFmpegDefaults;
};

export function FFmpegTab({ settings }: FFmpegTabProps) {
  return (
    <section id="panel-ffmpeg" role="tabpanel" aria-labelledby="tab-ffmpeg" className="tab-panel">
      <article className="panel-card panel-card-wide">
        <h3>Binary Configuration</h3>
        <div className="field-row">
          <label htmlFor="probe-binary">probe_binary</label>
          <div className="inline-field">
            <input id="probe-binary" name="probe-binary" value={settings.probeBinary} readOnly />
            <button type="button" className="btn btn-ghost">Test Path</button>
          </div>
          <p className="field-hint">Absolute path to the ffprobe executable. Ensure the server has execution permissions.</p>
        </div>
      </article>

      <article className="panel-card panel-card-wide">
        <div className="panel-card-head">
          <h3>probe_args</h3>
          <button type="button" className="text-action">Add Argument</button>
        </div>
        <p className="field-hint">Default command line flags used when analyzing media streams.</p>

        <div className="arg-table" role="table" aria-label="Probe arguments">
          <div className="arg-header" role="row">
            <span role="columnheader">FLAG</span>
            <span role="columnheader">VALUE</span>
            <span aria-hidden="true" />
          </div>
          {settings.probeArgs.map((item) => (
            <div key={item.id} className="arg-row" role="row">
              <input aria-label={`flag-${item.id}`} value={item.flag} readOnly />
              <input aria-label={`value-${item.id}`} value={item.value} readOnly />
              <button type="button" className="icon-btn" aria-label={`delete-${item.id}`}>🗑</button>
            </div>
          ))}
        </div>

        <div className="info-banner">
          Probing arguments directly affect metadata extraction performance. Using JSON output format is recommended for programmatic parsing.
        </div>
      </article>

      <div className="footer-actions footer-actions-spread">
        <button type="button" className="btn btn-secondary">Discard Changes</button>
        <button type="button" className="btn btn-primary">Save Configuration</button>
      </div>
    </section>
  );
}
