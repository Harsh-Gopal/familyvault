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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const electron_1 = require("electron");
const child_process_1 = require("child_process");
const path = __importStar(require("path"));
const fs = __importStar(require("fs"));
const os = __importStar(require("os"));
const axios_1 = __importDefault(require("axios"));
// Keep a global reference of the window object
let mainWindow = null;
let tray = null;
let backendProcess = null;
let backendPort = 8000;
let backendBaseURL = `http://127.0.0.1:${backendPort}`;
const isDev = process.env.NODE_ENV === 'development';
const isMac = process.platform === 'darwin';
// Single instance lock
const gotTheLock = electron_1.app.requestSingleInstanceLock();
if (!gotTheLock) {
    electron_1.app.quit();
}
else {
    electron_1.app.on('second-instance', () => {
        // Someone tried to run a second instance, focus our window instead
        if (mainWindow) {
            if (mainWindow.isMinimized())
                mainWindow.restore();
            mainWindow.focus();
        }
    });
    // This method will be called when Electron has finished initialization
    electron_1.app.whenReady().then(async () => {
        await createWindow();
        await startBackendIfNeeded();
        createTray();
        electron_1.app.on('activate', async () => {
            if (electron_1.BrowserWindow.getAllWindows().length === 0) {
                await createWindow();
            }
        });
    });
}
async function createWindow() {
    // Create the browser window
    mainWindow = new electron_1.BrowserWindow({
        width: 1280,
        height: 800,
        minWidth: 800,
        minHeight: 600,
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            sandbox: true,
            preload: path.join(__dirname, 'preload.cjs'),
            webSecurity: true,
        },
        titleBarStyle: isMac ? 'hiddenInset' : 'default',
        trafficLightPosition: isMac ? { x: 20, y: 20 } : undefined,
        show: false, // Don't show until ready
    });
    // Load the app
    if (isDev) {
        await mainWindow.loadURL('http://localhost:5173');
        mainWindow.webContents.openDevTools();
    }
    else {
        await mainWindow.loadFile(path.join(__dirname, '../index.html'));
        // DevTools disabled in production for clean user experience
    }
    // Show window when ready
    mainWindow.once('ready-to-show', () => {
        mainWindow?.show();
    });
    // Handle window closed
    mainWindow.on('closed', () => {
        mainWindow = null;
    });
    // Prevent navigation to external URLs
    mainWindow.webContents.on('will-navigate', (event, navigationUrl) => {
        const parsedUrl = new URL(navigationUrl);
        if (parsedUrl.origin !== 'http://localhost:5173' &&
            parsedUrl.origin !== 'file://' &&
            !parsedUrl.origin.startsWith('http://127.0.0.1')) {
            event.preventDefault();
        }
    });
    // Handle new window requests
    mainWindow.webContents.setWindowOpenHandler(({ url }) => {
        electron_1.shell.openExternal(url);
        return { action: 'deny' };
    });
    // Set up menu
    createMenu();
}
function createMenu() {
    const template = [
        ...(isMac ? [{
                label: electron_1.app.getName(),
                submenu: [
                    { role: 'about' },
                    { type: 'separator' },
                    { role: 'services' },
                    { type: 'separator' },
                    { role: 'hide' },
                    { role: 'hideOthers' },
                    { role: 'unhide' },
                    { type: 'separator' },
                    { role: 'quit' }
                ]
            }] : []),
        {
            label: 'File',
            submenu: [
                {
                    label: 'Open Vault Folder',
                    click: async () => {
                        try {
                            const health = await axios_1.default.get(`${backendBaseURL}/health`);
                            if (health.data.drive_path) {
                                electron_1.shell.showItemInFolder(health.data.drive_path);
                            }
                        }
                        catch (error) {
                            console.error('Failed to get drive path:', error);
                        }
                    }
                },
                { type: 'separator' },
                isMac ? { role: 'close' } : { role: 'quit' }
            ]
        },
        {
            label: 'Edit',
            submenu: [
                { role: 'undo' },
                { role: 'redo' },
                { type: 'separator' },
                { role: 'cut' },
                { role: 'copy' },
                { role: 'paste' },
                ...(isMac ? [
                    { role: 'pasteAndMatchStyle' },
                    { role: 'delete' },
                    { role: 'selectAll' },
                    { type: 'separator' },
                    {
                        label: 'Speech',
                        submenu: [
                            { role: 'startSpeaking' },
                            { role: 'stopSpeaking' }
                        ]
                    }
                ] : [
                    { role: 'delete' },
                    { type: 'separator' },
                    { role: 'selectAll' }
                ])
            ]
        },
        {
            label: 'View',
            submenu: [
                { role: 'reload' },
                { role: 'forceReload' },
                { role: 'toggleDevTools' },
                { type: 'separator' },
                { role: 'resetZoom' },
                { role: 'zoomIn' },
                { role: 'zoomOut' },
                { type: 'separator' },
                { role: 'togglefullscreen' }
            ]
        },
        {
            label: 'Window',
            submenu: [
                { role: 'minimize' },
                { role: 'close' },
                ...(isMac ? [
                    { type: 'separator' },
                    { role: 'front' },
                    { type: 'separator' },
                    { role: 'window' }
                ] : [])
            ]
        }
    ];
    const menu = electron_1.Menu.buildFromTemplate(template);
    electron_1.Menu.setApplicationMenu(menu);
}
function createTray() {
    // Create tray icon (you'll need to add an icon file)
    const iconPath = path.join(__dirname, '../assets/tray-icon.png');
    let trayIcon;
    if (fs.existsSync(iconPath)) {
        trayIcon = electron_1.nativeImage.createFromPath(iconPath);
    }
    else {
        // Fallback to a simple icon
        trayIcon = electron_1.nativeImage.createEmpty();
    }
    tray = new electron_1.Tray(trayIcon);
    const contextMenu = electron_1.Menu.buildFromTemplate([
        {
            label: 'Show FamilyVault',
            click: () => {
                if (mainWindow) {
                    mainWindow.show();
                    mainWindow.focus();
                }
            }
        },
        { type: 'separator' },
        {
            label: 'Open Vault Folder',
            click: async () => {
                try {
                    const health = await axios_1.default.get(`${backendBaseURL}/health`);
                    if (health.data.drive_path) {
                        electron_1.shell.showItemInFolder(health.data.drive_path);
                    }
                }
                catch (error) {
                    console.error('Failed to get drive path:', error);
                }
            }
        },
        { type: 'separator' },
        {
            label: 'Quit',
            click: () => {
                electron_1.app.quit();
            }
        }
    ]);
    tray.setToolTip('FamilyVault');
    tray.setContextMenu(contextMenu);
    tray.on('click', () => {
        if (mainWindow) {
            mainWindow.show();
            mainWindow.focus();
        }
    });
}
async function startBackendIfNeeded() {
    try {
        // Check if backend is already running
        await axios_1.default.get(`${backendBaseURL}/health`, { timeout: 2000 });
        console.log('Backend already running');
        return;
    }
    catch (error) {
        console.log('Backend not running, starting...');
    }
    // Find backend binary
    const backendPath = getBackendPath();
    if (!fs.existsSync(backendPath)) {
        throw new Error(`Backend binary not found at: ${backendPath}`);
    }
    // Set up environment
    const env = {
        ...process.env,
        FAMILYVAULT_DRIVE_PATH: path.join(os.homedir(), 'FamilyVault'),
        FAMILYVAULT_DATA_PATH: path.join(os.homedir(), '.familyvault'),
    };
    // Ensure directories exist
    fs.mkdirSync(env.FAMILYVAULT_DRIVE_PATH, { recursive: true });
    fs.mkdirSync(env.FAMILYVAULT_DATA_PATH, { recursive: true });
    // Start backend process
    backendProcess = (0, child_process_1.spawn)(backendPath, [], {
        env,
        stdio: ['ignore', 'pipe', 'pipe'],
        detached: false,
    });
    // Set up logging
    const logDir = path.join(os.homedir(), 'Library', 'Logs', 'FamilyVault');
    fs.mkdirSync(logDir, { recursive: true });
    const logFile = fs.createWriteStream(path.join(logDir, 'backend.log'), { flags: 'a' });
    backendProcess.stdout?.pipe(logFile);
    backendProcess.stderr?.pipe(logFile);
    backendProcess.on('error', (error) => {
        console.error('Backend process error:', error);
    });
    backendProcess.on('exit', (code) => {
        console.log(`Backend process exited with code ${code}`);
        backendProcess = null;
    });
    // Wait for backend to be ready
    await waitForBackend();
}
function getBackendPath() {
    if (process.env.FV_BACKEND_PATH) {
        return process.env.FV_BACKEND_PATH;
    }
    if (isDev) {
        return path.join(__dirname, '../../build/backend/familyvault');
    }
    else {
        return path.join(process.resourcesPath, 'app', 'build/backend/familyvault');
    }
}
async function waitForBackend(maxAttempts = 30) {
    for (let i = 0; i < maxAttempts; i++) {
        try {
            await axios_1.default.get(`${backendBaseURL}/health`, { timeout: 1000 });
            console.log('Backend is ready');
            return;
        }
        catch (error) {
            console.log(`Waiting for backend... (${i + 1}/${maxAttempts})`);
            await new Promise(resolve => setTimeout(resolve, 1000));
        }
    }
    throw new Error('Backend failed to start within timeout');
}
// App event handlers
electron_1.app.on('window-all-closed', () => {
    if (!isMac) {
        electron_1.app.quit();
    }
});
electron_1.app.on('before-quit', () => {
    if (backendProcess) {
        backendProcess.kill('SIGTERM');
        backendProcess = null;
    }
});
// Handle protocol for deep linking (familyvault://pair?token=...)
electron_1.app.setAsDefaultProtocolClient('familyvault');
electron_1.app.on('open-url', (event, url) => {
    event.preventDefault();
    if (mainWindow) {
        mainWindow.webContents.send('deep-link', url);
    }
});
// IPC handlers will be added in separate files
// Note: These will be renamed to .cjs by the build script
require("./ipc.cjs");
require("./keychain.cjs");
require("./backend.cjs");
require("./vault.cjs");
require("./sharing.cjs");
//# sourceMappingURL=main.js.map