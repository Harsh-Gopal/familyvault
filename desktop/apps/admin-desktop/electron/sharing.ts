import { ipcMain, shell } from 'electron';
import { exec } from 'child_process';
import { promisify } from 'util';
import * as fs from 'fs';
import * as path from 'path';

const execAsync = promisify(exec);

export interface ShareTarget {
  id: string;
  name: string;
  icon: string;
  available: boolean;
}

// Known app bundle IDs and their info
const KNOWN_APPS = [
  {
    id: 'mail',
    name: 'Mail',
    icon: '📧',
    bundleId: 'com.apple.mail',
    paths: ['/System/Applications/Mail.app']
  },
  {
    id: 'messages',
    name: 'Messages',
    icon: '💬',
    bundleId: 'com.apple.iChat',
    paths: ['/System/Applications/Messages.app']
  },
  {
    id: 'notes',
    name: 'Notes',
    icon: '📝',
    bundleId: 'com.apple.Notes',
    paths: ['/System/Applications/Notes.app']
  },
  {
    id: 'whatsapp',
    name: 'WhatsApp',
    icon: '📱',
    bundleId: 'WhatsApp',
    paths: ['/Applications/WhatsApp.app']
  },
  {
    id: 'telegram',
    name: 'Telegram',
    icon: '✈️',
    bundleId: 'ru.keepcoder.Telegram',
    paths: ['/Applications/Telegram.app']
  },
  {
    id: 'telegram-desktop',
    name: 'Telegram Desktop',
    icon: '✈️',
    bundleId: 'org.telegram.desktop',
    paths: ['/Applications/Telegram Desktop.app']
  },
  {
    id: 'copy',
    name: 'Copy to Clipboard',
    icon: '📋',
    bundleId: '',
    paths: []
  }
];

// Check if an app is installed
function isAppInstalled(appPaths: string[]): boolean {
  return appPaths.some(appPath => fs.existsSync(appPath));
}

// List available share targets
ipcMain.handle('share:listTargets', async (): Promise<{ ok: boolean; data?: ShareTarget[]; error?: string }> => {
  try {
    const targets: ShareTarget[] = [];
    
    for (const app of KNOWN_APPS) {
      const available = app.id === 'copy' || isAppInstalled(app.paths);
      
      targets.push({
        id: app.id,
        name: app.name,
        icon: app.icon,
        available
      });
    }
    
    // Only return available targets
    return { ok: true, data: targets.filter(t => t.available) };
  } catch (error) {
    console.error('Failed to list share targets:', error);
    return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
  }
});

// Invoke share target
ipcMain.handle('share:invoke', async (_, targetId: string, payload: string): Promise<{ ok: boolean; error?: string }> => {
  try {
    const app = KNOWN_APPS.find(a => a.id === targetId);
    if (!app) {
      return { ok: false, error: 'Unknown share target' };
    }
    
    switch (targetId) {
      case 'mail':
        await shell.openExternal(`mailto:?subject=${encodeURIComponent('FamilyVault Invite')}&body=${encodeURIComponent(payload)}`);
        break;
        
      case 'messages':
        try {
          // Use AppleScript to create a new message
          const script = `tell application "Messages"
            activate
            set newMessage to make new outgoing message with properties {content:"${payload.replace(/"/g, '\\"')}"}
          end tell`;
          await execAsync(`osascript -e '${script}'`);
        } catch (error) {
          console.warn('AppleScript failed, falling back to URL scheme');
          // Fallback - this might not work on all systems
          await shell.openExternal(`sms:&body=${encodeURIComponent(payload)}`);
        }
        break;
        
      case 'notes':
        try {
          // Use AppleScript to create a new note
          const script = `tell application "Notes"
            activate
            make new note with properties {body:"${payload.replace(/"/g, '\\"')}"}
          end tell`;
          await execAsync(`osascript -e '${script}'`);
        } catch (error) {
          console.warn('Failed to create note via AppleScript:', error);
          return { ok: false, error: 'Failed to create note' };
        }
        break;
        
      case 'whatsapp':
        await shell.openExternal(`whatsapp://send?text=${encodeURIComponent(payload)}`);
        break;
        
      case 'telegram':
      case 'telegram-desktop':
        await shell.openExternal(`tg://msg_url?text=${encodeURIComponent(payload)}`);
        break;
        
      case 'copy':
        // This will be handled in the renderer process
        return { ok: true };
        
      default:
        return { ok: false, error: 'Unsupported share target' };
    }
    
    return { ok: true };
  } catch (error) {
    console.error('Failed to invoke share target:', error);
    return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
  }
});