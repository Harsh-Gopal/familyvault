import { app, BrowserWindow, Menu, Tray, ipcMain, dialog, shell, nativeImage } from 'electron';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import axios from 'axios';

// Keep a global reference of the window object
let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let backendProcess: ChildProcess | null = null;
let backendPort = 8000;
let backendBaseURL = `http://127.0.0.1:${backendPort}`;

const isDev = process.env.NODE_ENV === 'development';
const isMac = process.platform === 'darwin';

// Single instance lock
const gotTheLock = app.requestSingleInstanceLock();

if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    // Someone tried to run a second instance, focus our window instead
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
  });

  // This method will be called when Electron has finished initialization
  app.whenReady().then(async () => {
    await createWindow();
    await startBackendIfNeeded();
    createTray();
    
    app.on('activate', async () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        await createWindow();
      }
    });
  });
}

async function createWindow(): Promise<void> {
  // Create the browser window
  mainWindow = new BrowserWindow({
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
  } else {
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
    shell.openExternal(url);
    return { action: 'deny' };
  });

  // Set up menu
  createMenu();
}

function createMenu(): void {
  const template: Electron.MenuItemConstructorOptions[] = [
    ...(isMac ? [{
      label: app.getName(),
      submenu: [
        { role: 'about' as const },
        { type: 'separator' as const },
        { role: 'services' as const },
        { type: 'separator' as const },
        { role: 'hide' as const },
        { role: 'hideOthers' as const },
        { role: 'unhide' as const },
        { type: 'separator' as const },
        { role: 'quit' as const }
      ]
    }] : []),
    {
      label: 'File',
      submenu: [
        {
          label: 'Open Vault Folder',
          click: async () => {
            try {
              const health = await axios.get(`${backendBaseURL}/health`);
              if (health.data.drive_path) {
                shell.showItemInFolder(health.data.drive_path);
              }
            } catch (error) {
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
          { role: 'pasteAndMatchStyle' as const },
          { role: 'delete' as const },
          { role: 'selectAll' as const },
          { type: 'separator' as const },
          {
            label: 'Speech',
            submenu: [
              { role: 'startSpeaking' as const },
              { role: 'stopSpeaking' as const }
            ]
          }
        ] : [
          { role: 'delete' as const },
          { type: 'separator' as const },
          { role: 'selectAll' as const }
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
          { type: 'separator' as const },
          { role: 'front' as const },
          { type: 'separator' as const },
          { role: 'window' as const }
        ] : [])
      ]
    }
  ];

  const menu = Menu.buildFromTemplate(template);
  Menu.setApplicationMenu(menu);
}

function createTray(): void {
  // Create tray icon (you'll need to add an icon file)
  const iconPath = path.join(__dirname, '../assets/tray-icon.png');
  let trayIcon: Electron.NativeImage;
  
  if (fs.existsSync(iconPath)) {
    trayIcon = nativeImage.createFromPath(iconPath);
  } else {
    // Fallback to a simple icon
    trayIcon = nativeImage.createEmpty();
  }
  
  tray = new Tray(trayIcon);
  
  const contextMenu = Menu.buildFromTemplate([
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
          const health = await axios.get(`${backendBaseURL}/health`);
          if (health.data.drive_path) {
            shell.showItemInFolder(health.data.drive_path);
          }
        } catch (error) {
          console.error('Failed to get drive path:', error);
        }
      }
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => {
        app.quit();
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

async function startBackendIfNeeded(): Promise<void> {
  try {
    // Check if backend is already running
    await axios.get(`${backendBaseURL}/health`, { timeout: 2000 });
    console.log('Backend already running');
    return;
  } catch (error) {
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
  fs.mkdirSync(env.FAMILYVAULT_DRIVE_PATH!, { recursive: true });
  fs.mkdirSync(env.FAMILYVAULT_DATA_PATH!, { recursive: true });

  // Start backend process
  backendProcess = spawn(backendPath, [], {
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

function getBackendPath(): string {
  if (process.env.FV_BACKEND_PATH) {
    return process.env.FV_BACKEND_PATH;
  }

  if (isDev) {
    return path.join(__dirname, '../../build/backend/familyvault');
  } else {
    return path.join(process.resourcesPath, 'app', 'build/backend/familyvault');
  }
}

async function waitForBackend(maxAttempts = 30): Promise<void> {
  for (let i = 0; i < maxAttempts; i++) {
    try {
      await axios.get(`${backendBaseURL}/health`, { timeout: 1000 });
      console.log('Backend is ready');
      return;
    } catch (error) {
      console.log(`Waiting for backend... (${i + 1}/${maxAttempts})`);
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
  }
  throw new Error('Backend failed to start within timeout');
}

// App event handlers
app.on('window-all-closed', () => {
  if (!isMac) {
    app.quit();
  }
});

app.on('before-quit', () => {
  if (backendProcess) {
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }
});

// Handle protocol for deep linking (familyvault://pair?token=...)
app.setAsDefaultProtocolClient('familyvault');

app.on('open-url', (event, url) => {
  event.preventDefault();
  if (mainWindow) {
    mainWindow.webContents.send('deep-link', url);
  }
});

// IPC handlers will be added in separate files
// Note: These will be renamed to .cjs by the build script
import './ipc';
import './keychain';
import './backend';
import './vault';
import './sharing';