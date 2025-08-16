import { contextBridge, ipcRenderer } from 'electron';

export interface DeviceInfo {
  id: string;
  name: string;
  mountPoint: string;
  type: 'internal' | 'external' | 'network';
  isRemovable: boolean;
  capacity: number;
  free: number;
  usedPct: number;
}

export interface ShareTarget {
  id: string;
  name: string;
  icon: string;
  available: boolean;
}

export interface VaultAPI {
  listDevices(): Promise<{ ok: boolean; data?: DeviceInfo[]; error?: string }>;
  chooseFolder(): Promise<{ ok: boolean; data?: string; error?: string }>;
  setSelection(groupId: string, userId: string, mountPoint: string): Promise<{ ok: boolean; data?: string; error?: string }>;
  getAssignment(groupId: string, userId: string): Promise<{ ok: boolean; data?: any; error?: string }>;
  copyFiles(groupId: string, userId: string, filePaths: string[]): Promise<{ ok: boolean; error?: string }>;
  openInFinder(groupId: string, userId: string): Promise<{ ok: boolean; error?: string }>;
  listFiles(groupId: string, userId: string): Promise<{ ok: boolean; data?: any[]; error?: string }>;
  downloadFile(groupId: string, userId: string, fileId: string): Promise<{ ok: boolean; error?: string }>;
  updateQuota(groupId: string, userId: string, quotaBytes: number): Promise<{ ok: boolean; error?: string }>;
  onCopyProgress(callback: (progress: any) => void): void;
}

export interface ShareAPI {
  listTargets(): Promise<{ ok: boolean; data?: ShareTarget[]; error?: string }>;
  invoke(targetId: string, payload: string): Promise<{ ok: boolean; error?: string }>;
}

export interface FVBridge {
  getVersion(): Promise<string>;
  spawnBackendIfNeeded(): Promise<void>;
  getBaseURL(): Promise<string>;
  setToken(token: string): Promise<void>;
  getToken(): Promise<string | null>;
  clearToken(): Promise<void>;
  openFileDialog(options: Electron.OpenDialogOptions): Promise<string[]>;
  showItemInFolder(path: string): void;
  copyToClipboard(text: string): void;
  onDeepLink(callback: (url: string) => void): void;
  removeAllListeners(channel: string): void;
  vault: VaultAPI;
  share: ShareAPI;
}

const fvBridge: FVBridge = {
  getVersion: () => ipcRenderer.invoke('get-version'),
  spawnBackendIfNeeded: () => ipcRenderer.invoke('spawn-backend'),
  getBaseURL: () => ipcRenderer.invoke('get-base-url'),
  setToken: (token: string) => ipcRenderer.invoke('set-token', token),
  getToken: () => ipcRenderer.invoke('get-token'),
  clearToken: () => ipcRenderer.invoke('clear-token'),
  openFileDialog: (options: Electron.OpenDialogOptions) => ipcRenderer.invoke('open-file-dialog', options),
  showItemInFolder: (path: string) => ipcRenderer.invoke('show-item-in-folder', path),
  copyToClipboard: (text: string) => ipcRenderer.invoke('copy-to-clipboard', text),
  onDeepLink: (callback: (url: string) => void) => {
    ipcRenderer.on('deep-link', (_, url) => callback(url));
  },
  removeAllListeners: (channel: string) => {
    ipcRenderer.removeAllListeners(channel);
  },
  vault: {
    listDevices: () => ipcRenderer.invoke('vault:listDevices'),
    chooseFolder: () => ipcRenderer.invoke('vault:chooseFolder'),
    setSelection: (groupId: string, userId: string, mountPoint: string) => 
      ipcRenderer.invoke('vault:setSelection', groupId, userId, mountPoint),
    getAssignment: (groupId: string, userId: string) => 
      ipcRenderer.invoke('vault:getAssignment', groupId, userId),
    copyFiles: (groupId: string, userId: string, filePaths: string[]) => 
      ipcRenderer.invoke('vault:copyFiles', groupId, userId, filePaths),
    openInFinder: (groupId: string, userId: string) => 
      ipcRenderer.invoke('vault:openInFinder', groupId, userId),
    listFiles: (groupId: string, userId: string) => 
      ipcRenderer.invoke('vault:listFiles', groupId, userId),
    downloadFile: (groupId: string, userId: string, fileId: string) => 
      ipcRenderer.invoke('vault:downloadFile', groupId, userId, fileId),
    updateQuota: (groupId: string, userId: string, quotaBytes: number) => 
      ipcRenderer.invoke('vault:updateQuota', groupId, userId, quotaBytes),
    onCopyProgress: (callback: (progress: any) => void) => {
      ipcRenderer.on('vault:copyProgress', (_, progress) => callback(progress));
    }
  },
  share: {
    listTargets: () => ipcRenderer.invoke('share:listTargets'),
    invoke: (targetId: string, payload: string) => 
      ipcRenderer.invoke('share:invoke', targetId, payload)
  }
};

contextBridge.exposeInMainWorld('fv', fvBridge);

// Type declaration for the global window object
declare global {
  interface Window {
    fv: FVBridge;
  }
}