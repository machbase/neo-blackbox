import type { LogSettings } from '../types/settings';

type LogTabProps = {
  settings: LogSettings;
};

export function LogTab({ settings }: LogTabProps) {
  return (
    <section id="panel-log" role="tabpanel" aria-labelledby="tab-log" className="tab-panel">
      <article className="panel-card panel-card-wide">
        <h3>General Logging</h3>

        <div className="two-column-fields">
          <div className="field-row">
            <label htmlFor="log-directory">Log Directory</label>
            <input id="log-directory" name="log-directory" value={settings.logDirectory} readOnly />
          </div>

          <div className="field-row">
            <label htmlFor="log-level">Log Level</label>
            <select id="log-level" name="log-level" value={settings.logLevel} disabled>
              <option value="debug">debug</option>
              <option value="info">info</option>
              <option value="warn">warn</option>
              <option value="error">error</option>
            </select>
          </div>

          <div className="field-row">
            <label htmlFor="log-format">Log Format</label>
            <select id="log-format" name="log-format" value={settings.logFormat} disabled>
              <option value="json">JSON</option>
              <option value="text">Text</option>
            </select>
          </div>

          <div className="field-row">
            <label htmlFor="output-destination">Output Destination</label>
            <select id="output-destination" name="output-destination" value={settings.outputDestination} disabled>
              <option value="stdout">Stdout</option>
              <option value="file">File</option>
              <option value="both">Both</option>
            </select>
          </div>
        </div>
      </article>

      <article className="panel-card panel-card-wide">
        <h3>File Retention &amp; Rotation</h3>

        <div className="two-column-fields">
          <div className="field-row field-span-2">
            <label htmlFor="filename-prefix">Filename Pattern</label>
            <div className="inline-field compact-inline">
              <input id="filename-prefix" name="filename-prefix" value={settings.filenamePrefix} readOnly />
              <span className="suffix-pill">.{settings.filenameExtension}</span>
            </div>
          </div>

          <div className="field-row">
            <label htmlFor="max-file-size">Max File Size (MB)</label>
            <input id="max-file-size" name="max-file-size" value={settings.maxFileSizeMb} readOnly />
          </div>

          <div className="field-row">
            <label htmlFor="max-backups">Max Backups</label>
            <input id="max-backups" name="max-backups" value={settings.maxBackups} readOnly />
          </div>

          <div className="field-row">
            <label htmlFor="max-age-days">Max Age (Days)</label>
            <input id="max-age-days" name="max-age-days" value={settings.maxAgeDays} readOnly />
          </div>
        </div>

        <label className="toggle-row" htmlFor="compress-logs">
          <div>
            <p className="toggle-title">Compress Old Logs</p>
            <p className="field-hint">Automatically compress old log files to save disk space.</p>
          </div>
          <span className="toggle-control">
            <input
              id="compress-logs"
              name="compress-logs"
              type="checkbox"
              checked={settings.compressOldLogs}
              readOnly
            />
            <span className="toggle-slider" />
          </span>
        </label>
      </article>

      <div className="footer-actions footer-actions-spread">
        <button type="button" className="btn btn-secondary">Discard Changes</button>
        <button type="button" className="btn btn-primary">Update Configuration</button>
      </div>
    </section>
  );
}
