"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const electron_1 = require("electron");
const fvBridge = {
    getVersion: () => electron_1.ipcRenderer.invoke('get-version'),
    spawnBackendIfNeeded: () => electron_1.ipcRenderer.invoke('spawn-backend'),
    getBaseURL: () => electron_1.ipcRenderer.invoke('get-base-url'),
    setToken: (token) => electron_1.ipcRenderer.invoke('set-token', token),
    getToken: () => electron_1.ipcRenderer.invoke('get-token'),
    clearToken: () => electron_1.ipcRenderer.invoke('clear-token'),
    openFileDialog: (options) => electron_1.ipcRenderer.invoke('open-file-dialog', options),
    showItemInFolder: (path) => electron_1.ipcRenderer.invoke('show-item-in-folder', path),
    copyToClipboard: (text) => electron_1.ipcRenderer.invoke('copy-to-clipboard', text),
    onDeepLink: (callback) => {
        electron_1.ipcRenderer.on('deep-link', (_, url) => callback(url));
    },
    removeAllListeners: (channel) => {
        electron_1.ipcRenderer.removeAllListeners(channel);
    },
    vault: {
        listDevices: () => electron_1.ipcRenderer.invoke('vault:listDevices'),
        chooseFolder: () => electron_1.ipcRenderer.invoke('vault:chooseFolder'),
        setSelection: (groupId, userId, mountPoint) => electron_1.ipcRenderer.invoke('vault:setSelection', groupId, userId, mountPoint),
        getAssignment: (groupId, userId) => electron_1.ipcRenderer.invoke('vault:getAssignment', groupId, userId),
        copyFiles: (groupId, userId, filePaths) => electron_1.ipcRenderer.invoke('vault:copyFiles', groupId, userId, filePaths),
        openInFinder: (groupId, userId) => electron_1.ipcRenderer.invoke('vault:openInFinder', groupId, userId),
        listFiles: (groupId, userId) => electron_1.ipcRenderer.invoke('vault:listFiles', groupId, userId),
        downloadFile: (groupId, userId, fileId) => electron_1.ipcRenderer.invoke('vault:downloadFile', groupId, userId, fileId),
        updateQuota: (groupId, userId, quotaBytes) => electron_1.ipcRenderer.invoke('vault:updateQuota', groupId, userId, quotaBytes),
        onCopyProgress: (callback) => {
            electron_1.ipcRenderer.on('vault:copyProgress', (_, progress) => callback(progress));
        }
    },
    share: {
        listTargets: () => electron_1.ipcRenderer.invoke('share:listTargets'),
        invoke: (targetId, payload) => electron_1.ipcRenderer.invoke('share:invoke', targetId, payload)
    }
};
electron_1.contextBridge.exposeInMainWorld('fv', fvBridge);
//# sourceMappingURL=preload.js.map