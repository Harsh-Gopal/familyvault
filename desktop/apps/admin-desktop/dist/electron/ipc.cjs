"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const electron_1 = require("electron");
// Version info
electron_1.ipcMain.handle('get-version', () => {
    return electron_1.app.getVersion();
});
// Backend management
electron_1.ipcMain.handle('spawn-backend', async () => {
    // Backend spawning is handled in main.ts
    return Promise.resolve();
});
electron_1.ipcMain.handle('get-base-url', () => {
    return 'http://127.0.0.1:8000';
});
// File operations
electron_1.ipcMain.handle('open-file-dialog', async (_, options) => {
    const result = await electron_1.dialog.showOpenDialog(options);
    return result.filePaths;
});
electron_1.ipcMain.handle('show-item-in-folder', (_, path) => {
    electron_1.shell.showItemInFolder(path);
});
// Clipboard
electron_1.ipcMain.handle('copy-to-clipboard', (_, text) => {
    electron_1.clipboard.writeText(text);
});
//# sourceMappingURL=ipc.js.map