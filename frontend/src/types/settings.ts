export type SettingsTab = 'general' | 'ffmpeg' | 'log';

export interface ServerPaths {
  address: string;
  cameraDirectory: string;
  mvsDirectory: string;
  dataDirectory: string;
}

export interface MachbaseSettings {
  host: string;
  port: number;
  timeoutSeconds: number;
}

export interface MediaMtxSettings {
  host: string;
  port: number;
  binary: string;
}

export interface FFmpegBinarySettings {
  binary: string;
  probeBinary: string;
}

export interface GeneralSettings {
  server: ServerPaths;
  machbase: MachbaseSettings;
  mediaMtx: MediaMtxSettings;
  ffmpeg: FFmpegBinarySettings;
}

export interface ProbeArgItem {
  id: string;
  flag: string;
  value: string;
}

export interface FFmpegDefaults {
  probeBinary: string;
  probeArgs: ProbeArgItem[];
}

export interface LogSettings {
  logDirectory: string;
  logLevel: 'debug' | 'info' | 'warn' | 'error';
  logFormat: 'json' | 'text';
  outputDestination: 'stdout' | 'file' | 'both';
  filenamePrefix: string;
  filenameExtension: string;
  maxFileSizeMb: number;
  maxBackups: number;
  maxAgeDays: number;
  compressOldLogs: boolean;
}
