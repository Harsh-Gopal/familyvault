import { ipcMain } from 'electron';
import * as keytar from 'keytar';

const SERVICE_NAME = 'FamilyVault';
const ACCOUNT_NAME = 'auth-token';

// Token management
ipcMain.handle('set-token', async (_, token: string) => {
  try {
    await keytar.setPassword(SERVICE_NAME, ACCOUNT_NAME, token);
    return true;
  } catch (error) {
    console.error('Failed to store token in keychain:', error);
    throw error;
  }
});

ipcMain.handle('get-token', async () => {
  try {
    return await keytar.getPassword(SERVICE_NAME, ACCOUNT_NAME);
  } catch (error) {
    console.error('Failed to retrieve token from keychain:', error);
    return null;
  }
});

ipcMain.handle('clear-token', async () => {
  try {
    return await keytar.deletePassword(SERVICE_NAME, ACCOUNT_NAME);
  } catch (error) {
    console.error('Failed to clear token from keychain:', error);
    throw error;
  }
});