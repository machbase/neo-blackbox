import type { LogSettings } from '../types/settings';

type LogTabProps = {
  settings: LogSettings;
  onChange: (next: LogSettings) => void;
};

function toNumber(raw: string, fallback: number): number {
  const parsed = Number(raw);
  return Number.isNaN(parsed) ? fallback : parsed;
}

export function LogTab({ settings, onChange }: LogTabProps) {
  return (
    <section id="panel-log" role="tabpanel" aria-labelledby="tab-log" className="tab-panel">
      <article className="panel-card panel-card-wide">
        <h3>General Logging</h3>

        <div className="two-column-fields">
          <div className="field-row">
            <label htmlFor="log-directory">Log Directory</label>
            <input
              id="log-directory"
              name="log-directory"
              value={settings.logDirectory}
              onChange={(event) => onChange({ ...settings, logDirectory: event.target.value })}
            />
          </div>

          <div className="field-row">
            <label htmlFor="log-level">Log Level</label>
            <select
              id="log-level"
              name="log-level"
              value={settings.logLevel}
              onChange={(event) =>
                onChange({ ...settings, logLevel: event.target.value as LogSettings['logLevel'] })}
            >
              <option value="debug">debug</option>
              <option value="info">info</option>
              <option value="warn">warn</option>
              <option value="error">error</option>
            </select>
          </div>

          <div className="field-row">
            <label htmlFor="log-format">Log Format</label>
            <select
              id="log-format"
              name="log-format"
              value={settings.logFormat}
              onChange={(event) =>
                onChange({ ...settings, logFormat: event.target.value as LogSettings['logFormat'] })}
            >
              <option value="json">JSON</option>
              <option value="text">Text</option>
            </select>
          </div>

          <div className="field-row">
            <label htmlFor="output-destination">Output Destination</label>
            <select
              id="output-destination"
              name="output-destination"
              value={settings.outputDestination}
              onChange={(event) =>
                onChange({ ...settings, outputDestination: event.target.value as LogSettings['outputDestination'] })}
            >
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
          <div className="field-row">
            <label htmlFor="filename-prefix">Filename Pattern</label>
            <div className="inline-field compact-inline filename-pattern-wrap">
              <input
                id="filename-prefix"
                name="filename-prefix"
                className="filename-pattern-input"
                value={settings.filenamePrefix}
                onChange={(event) => onChange({ ...settings, filenamePrefix: event.target.value })}
              />
              <span className="suffix-pill filename-suffix">.log</span>
            </div>
          </div>

          <div className="field-row">
            <label htmlFor="max-file-size">Max File Size (MB)</label>
            <input
              id="max-file-size"
              name="max-file-size"
              type="number"
              value={settings.maxFileSizeMb}
              onChange={(event) =>
                onChange({ ...settings, maxFileSizeMb: toNumber(event.target.value, settings.maxFileSizeMb) })}
            />
          </div>

          <div className="field-row">
            <label htmlFor="max-backups">Max Backups</label>
            <input
              id="max-backups"
              name="max-backups"
              type="number"
              value={settings.maxBackups}
              onChange={(event) => onChange({ ...settings, maxBackups: toNumber(event.target.value, settings.maxBackups) })}
            />
          </div>

          <div className="field-row">
            <label htmlFor="max-age-days">Max Age (Days)</label>
            <input
              id="max-age-days"
              name="max-age-days"
              type="number"
              value={settings.maxAgeDays}
              onChange={(event) => onChange({ ...settings, maxAgeDays: toNumber(event.target.value, settings.maxAgeDays) })}
            />
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
              onChange={(event) => onChange({ ...settings, compressOldLogs: event.target.checked })}
            />
            <span className="toggle-slider" />
          </span>
        </label>
      </article>
    </section>
  );
}
