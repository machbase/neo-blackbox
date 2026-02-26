import type { GeneralSettings } from '../types/settings';

type GeneralTabProps = {
  settings: GeneralSettings;
};

type ReadonlyFieldProps = {
  id: string;
  label: string;
  value: string | number;
};

function ReadonlyField({ id, label, value }: ReadonlyFieldProps) {
  return (
    <div className="field-row">
      <label htmlFor={id}>{label}</label>
      <input id={id} name={id} value={value} readOnly />
    </div>
  );
}

export function GeneralTab({ settings }: GeneralTabProps) {
  return (
    <section id="panel-general" role="tabpanel" aria-labelledby="tab-general" className="tab-panel">
      <div className="card-grid">
        <article className="panel-card">
          <h3>Server</h3>
          <ReadonlyField id="server-address" label="Address" value={settings.server.address} />
          <ReadonlyField id="server-camera-dir" label="Camera Directory" value={settings.server.cameraDirectory} />
          <ReadonlyField id="server-mvs-dir" label="MVS Directory" value={settings.server.mvsDirectory} />
          <ReadonlyField id="server-data-dir" label="Data Directory" value={settings.server.dataDirectory} />
        </article>

        <article className="panel-card">
          <h3>Machbase</h3>
          <ReadonlyField id="machbase-host" label="Host" value={settings.machbase.host} />
          <ReadonlyField id="machbase-port" label="Port" value={settings.machbase.port} />
          <ReadonlyField id="machbase-timeout" label="Timeout Seconds" value={settings.machbase.timeoutSeconds} />
        </article>

        <article className="panel-card">
          <h3>MediaMTX</h3>
          <ReadonlyField id="mediamtx-host" label="Host" value={settings.mediaMtx.host} />
          <ReadonlyField id="mediamtx-port" label="Port" value={settings.mediaMtx.port} />
          <ReadonlyField id="mediamtx-binary" label="Binary" value={settings.mediaMtx.binary} />
        </article>

        <article className="panel-card">
          <h3>FFmpeg</h3>
          <ReadonlyField id="ffmpeg-binary" label="Binary" value={settings.ffmpeg.binary} />
          <ReadonlyField id="ffmpeg-probe-binary" label="FFprobe Binary" value={settings.ffmpeg.probeBinary} />
        </article>
      </div>

      <div className="footer-actions">
        <button type="button" className="btn btn-primary">Save Changes</button>
      </div>
    </section>
  );
}
