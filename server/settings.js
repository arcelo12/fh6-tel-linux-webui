import fs from 'fs';
import path from 'path';
import os from 'os';

function settingsPath() {
  const localAppdata = process.env.LOCALAPPDATA || path.join(os.homedir(), '.local', 'share');
  return path.join(localAppdata, 'fh6-tel', 'settings.json');
}

export function getDefaultSettings() {
  return {
    port: 20440,
    useMph: true,
    tireTempCold: 60.0,
    tireTempOptimal: 85.0,
    tireTempHot: 110.0,
    autoRecord: true,
    theme: 'dark',
    mapEnabled: false,
    mapOverride: false,
    mapTileUrl: '',
    mapMinZoom: 0,
    mapMaxZoom: 5,
    mapTileSize: 256,
    mapCalAWorld: [0.0, 0.0],
    mapCalAPix: [0.0, 0.0],
    mapCalBWorld: [0.0, 0.0],
    mapCalBPix: [0.0, 0.0],
    mapViewMaxZoom: 0,
    mapDefaultZoom: 0,
    mapDefaultCenter: [0.0, 0.0],
  };
}

export function loadSettings() {
  const p = settingsPath();
  try {
    const raw = fs.readFileSync(p, 'utf8');
    const parsed = JSON.parse(raw);
    return { ...getDefaultSettings(), ...parsed };
  } catch (e) {
    return getDefaultSettings();
  }
}

export function saveSettings(settings) {
  const p = settingsPath();
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, JSON.stringify(settings, null, 2), 'utf8');
}
