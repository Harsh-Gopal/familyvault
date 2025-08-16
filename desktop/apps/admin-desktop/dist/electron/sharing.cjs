"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
const electron_1 = require("electron");
const child_process_1 = require("child_process");
const util_1 = require("util");
const fs = __importStar(require("fs"));
const execAsync = (0, util_1.promisify)(child_process_1.exec);
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
function isAppInstalled(appPaths) {
    return appPaths.some(appPath => fs.existsSync(appPath));
}
// List available share targets
electron_1.ipcMain.handle('share:listTargets', async () => {
    try {
        const targets = [];
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
    }
    catch (error) {
        console.error('Failed to list share targets:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Invoke share target
electron_1.ipcMain.handle('share:invoke', async (_, targetId, payload) => {
    try {
        const app = KNOWN_APPS.find(a => a.id === targetId);
        if (!app) {
            return { ok: false, error: 'Unknown share target' };
        }
        switch (targetId) {
            case 'mail':
                await electron_1.shell.openExternal(`mailto:?subject=${encodeURIComponent('FamilyVault Invite')}&body=${encodeURIComponent(payload)}`);
                break;
            case 'messages':
                try {
                    // Use AppleScript to create a new message
                    const script = `tell application "Messages"
            activate
            set newMessage to make new outgoing message with properties {content:"${payload.replace(/"/g, '\\"')}"}
          end tell`;
                    await execAsync(`osascript -e '${script}'`);
                }
                catch (error) {
                    console.warn('AppleScript failed, falling back to URL scheme');
                    // Fallback - this might not work on all systems
                    await electron_1.shell.openExternal(`sms:&body=${encodeURIComponent(payload)}`);
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
                }
                catch (error) {
                    console.warn('Failed to create note via AppleScript:', error);
                    return { ok: false, error: 'Failed to create note' };
                }
                break;
            case 'whatsapp':
                await electron_1.shell.openExternal(`whatsapp://send?text=${encodeURIComponent(payload)}`);
                break;
            case 'telegram':
            case 'telegram-desktop':
                await electron_1.shell.openExternal(`tg://msg_url?text=${encodeURIComponent(payload)}`);
                break;
            case 'copy':
                // This will be handled in the renderer process
                return { ok: true };
            default:
                return { ok: false, error: 'Unsupported share target' };
        }
        return { ok: true };
    }
    catch (error) {
        console.error('Failed to invoke share target:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
//# sourceMappingURL=sharing.js.map