import { ipcMain, dialog, shell, clipboard, app } from 'electron';

// Version info
ipcMain.handle('get-version', () => {
  return app.getVersion();
});

// Backend management
ipcMain.handle('spawn-backend', async () => {
  // Backend spawning is handled in main.ts
  return Promise.resolve();
});

ipcMain.handle('get-base-url', () => {
  return 'http://127.0.0.1:8000';
});

// File operations
ipcMain.handle('open-file-dialog', async (_, options: Electron.OpenDialogOptions) => {
  const result = await dialog.showOpenDialog(options);
  return result.filePaths;
});

ipcMain.handle('show-item-in-folder', (_, path: string) => {
  shell.showItemInFolder(path);
});

// Clipboard
ipcMain.handle('copy-to-clipboard', (_, text: string) => {
  clipboard.writeText(text);
});