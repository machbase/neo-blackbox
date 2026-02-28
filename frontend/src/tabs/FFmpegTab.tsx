import type { FFmpegDefaults } from '../types/settings';

type FFmpegTabProps = {
  settings: FFmpegDefaults;
  onChange: (next: FFmpegDefaults) => void;
};

function createProbeArgId(): string {
  return `arg-${Date.now()}-${Math.random().toString(16).slice(2, 6)}`;
}

export function FFmpegTab({ settings, onChange }: FFmpegTabProps) {
  const updateProbeArg = (id: string, key: 'flag' | 'value', value: string) => {
    onChange({
      ...settings,
      probeArgs: settings.probeArgs.map((item) => (
        item.id === id ? { ...item, [key]: value } : item
      )),
    });
  };

  const addProbeArg = () => {
    onChange({
      ...settings,
      probeArgs: [
        ...settings.probeArgs,
        { id: createProbeArgId(), flag: '', value: '' },
      ],
    });
  };

  const removeProbeArg = (id: string) => {
    onChange({
      ...settings,
      probeArgs: settings.probeArgs.filter((item) => item.id !== id),
    });
  };

  return (
    <section id="panel-ffmpeg" role="tabpanel" aria-labelledby="tab-ffmpeg" className="tab-panel">
      <article className="panel-card panel-card-wide">
        <div className="panel-card-head">
          <h3>probe_args</h3>
          <button type="button" className="text-action" onClick={addProbeArg}>Add Argument</button>
        </div>
        <p className="field-hint">Default command line flags used when analyzing media streams.</p>

        <div className="probe-args-list" aria-label="Probe arguments">
          {settings.probeArgs.map((item) => (
            <div key={item.id} className="probe-arg-card">
              <div className="probe-arg-fields">
                <div className="probe-arg-field">
                  <label htmlFor={`flag-${item.id}`}>FLAG</label>
                  <input
                    id={`flag-${item.id}`}
                    name={`flag-${item.id}`}
                    value={item.flag}
                    onChange={(event) => updateProbeArg(item.id, 'flag', event.target.value)}
                  />
                </div>
                <div className="probe-arg-field">
                  <label htmlFor={`value-${item.id}`}>VALUE</label>
                  <input
                    id={`value-${item.id}`}
                    name={`value-${item.id}`}
                    value={item.value}
                    onChange={(event) => updateProbeArg(item.id, 'value', event.target.value)}
                  />
                </div>
              </div>
              <button
                type="button"
                className="icon-btn"
                aria-label={`delete-${item.id}`}
                onClick={() => removeProbeArg(item.id)}
              >
                🗑
              </button>
            </div>
          ))}
        </div>

        <div className="info-banner">
          Probing arguments directly affect metadata extraction performance. Using JSON output format is recommended for programmatic parsing.
        </div>
      </article>
    </section>
  );
}
