import type { FFmpegDefaults, GeneralSettings, LogSettings } from '../types/settings';

export const generalSettings: GeneralSettings = {
  server: {
    address: '0.0.0.0:8000',
    cameraDirectory: '/blackbox/be/cameras',
    mvsDirectory: '/blackbox/ai/mvs',
    dataDirectory: '/blackbox/be/data',
  },
  machbase: {
    host: '127.0.0.1',
    port: 5654,
    timeoutSeconds: 15,
  },
  mediaMtx: {
    host: '127.0.0.1',
    port: 9997,
    binary: 'mediamtx',
  },
  ffmpeg: {
    binary: 'ffmpeg',
    probeBinary: 'ffprobe',
  },
};

export const ffmpegDefaults: FFmpegDefaults = {
  probeBinary: '/usr/bin/ffprobe',
  probeArgs: [
    { id: 'arg-1', flag: '-v', value: 'error' },
    { id: 'arg-2', flag: '-select_streams', value: 'v:0' },
    { id: 'arg-3', flag: '-show_entries', value: 'packet=pts_time,duration_time' },
    { id: 'arg-4', flag: '-of', value: 'csv=p=0' },
  ],
};

export const logSettings: LogSettings = {
  logDirectory: '/blackbox/be/logs',
  logLevel: 'info',
  logFormat: 'json',
  outputDestination: 'both',
  filenamePrefix: 'blackbox',
  filenameExtension: 'log',
  maxFileSizeMb: 100,
  maxBackups: 10,
  maxAgeDays: 30,
  compressOldLogs: true,
};
