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
const drivelist = __importStar(require("drivelist"));
const diskusage = __importStar(require("diskusage"));
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const crypto = __importStar(require("crypto"));
const execAsync = (0, util_1.promisify)(child_process_1.exec);
let config = { storage: {} };
const configPath = path.join(electron_1.app.getPath('userData'), 'vault-config.json');
// Load config on startup
function loadConfig() {
    try {
        if (fs.existsSync(configPath)) {
            const data = fs.readFileSync(configPath, 'utf8');
            return JSON.parse(data);
        }
    }
    catch (error) {
        console.error('Failed to load vault config:', error);
    }
    return { storage: {} };
}
// Save config
function saveConfig(newConfig) {
    try {
        config = newConfig;
        fs.writeFileSync(configPath, JSON.stringify(config, null, 2));
    }
    catch (error) {
        console.error('Failed to save vault config:', error);
        throw error;
    }
}
// Initialize config
config = loadConfig();
// Get device information using diskutil on macOS
async function getMacOSDeviceInfo(mountPoint) {
    try {
        const { stdout } = await execAsync(`diskutil info -plist "${mountPoint}"`);
        // Parse plist output - simplified version
        const isRemovable = stdout.includes('<key>Removable</key>') && stdout.includes('<true/>');
        const isInternal = stdout.includes('<key>Internal</key>') && stdout.includes('<true/>');
        return {
            type: isRemovable ? 'external' : (isInternal ? 'internal' : 'network'),
            isRemovable
        };
    }
    catch (error) {
        console.warn('Failed to get macOS device info:', error);
        return { type: 'internal', isRemovable: false };
    }
}
// List all available devices
electron_1.ipcMain.handle('vault:listDevices', async () => {
    try {
        const devices = [];
        try {
            // Try to get physical drives using drivelist
            const drives = await drivelist.list();
            for (const drive of drives) {
                for (const mountpoint of drive.mountpoints) {
                    if (!mountpoint.path)
                        continue;
                    try {
                        const usage = await diskusage.check(mountpoint.path);
                        const macInfo = process.platform === 'darwin'
                            ? await getMacOSDeviceInfo(mountpoint.path)
                            : { type: 'internal', isRemovable: false };
                        const device = {
                            id: `${drive.device}-${mountpoint.path}`,
                            name: mountpoint.label || path.basename(mountpoint.path) || drive.description || 'Unknown Drive',
                            mountPoint: mountpoint.path,
                            type: macInfo.type || 'internal',
                            isRemovable: macInfo.isRemovable || false,
                            capacity: usage.total,
                            free: usage.free,
                            usedPct: Math.round(((usage.total - usage.free) / usage.total) * 100)
                        };
                        devices.push(device);
                    }
                    catch (error) {
                        console.warn(`Failed to get usage for ${mountpoint.path}:`, error);
                    }
                }
            }
        }
        catch (drivelistError) {
            console.warn('Drivelist failed, falling back to diskutil:', drivelistError);
            // Fallback: Use df command and system_profiler on macOS
            if (process.platform === 'darwin') {
                try {
                    // Get all mounted volumes
                    const { stdout } = await execAsync('df -h');
                    const lines = stdout.split('\n').slice(1); // Skip header
                    // Also try to get USB device info
                    let usbDevices = [];
                    try {
                        const { stdout: usbInfo } = await execAsync('system_profiler SPUSBDataType -json');
                        const usbData = JSON.parse(usbInfo);
                        usbDevices = usbData.SPUSBDataType || [];
                    }
                    catch (usbError) {
                        console.warn('Failed to get USB device info:', usbError);
                    }
                    for (const line of lines) {
                        const parts = line.trim().split(/\s+/);
                        if (parts.length >= 6 && parts[5].startsWith('/')) {
                            const mountPoint = parts[5];
                            if (mountPoint === '/' || mountPoint.startsWith('/Volumes/')) {
                                try {
                                    const usage = await diskusage.check(mountPoint);
                                    const macInfo = await getMacOSDeviceInfo(mountPoint);
                                    // Check if this is a USB device
                                    const volumeName = path.basename(mountPoint);
                                    const isUSB = mountPoint.startsWith('/Volumes/') && (macInfo.isRemovable ||
                                        usbDevices.some(usb => usb._name && volumeName.toLowerCase().includes(usb._name.toLowerCase().split(' ')[0])));
                                    const device = {
                                        id: `fallback-${mountPoint.replace(/[^a-zA-Z0-9]/g, '-')}`,
                                        name: mountPoint === '/' ? 'Macintosh HD' : volumeName,
                                        mountPoint: mountPoint,
                                        type: isUSB ? 'external' : (macInfo.type || 'internal'),
                                        isRemovable: isUSB || macInfo.isRemovable || false,
                                        capacity: usage.total,
                                        free: usage.free,
                                        usedPct: Math.round(((usage.total - usage.free) / usage.total) * 100)
                                    };
                                    devices.push(device);
                                }
                                catch (error) {
                                    console.warn(`Failed to get usage for ${mountPoint}:`, error);
                                }
                            }
                        }
                    }
                }
                catch (dfError) {
                    console.warn('df command also failed:', dfError);
                    // Final fallback: just scan /Volumes directory
                    try {
                        const volumesDir = '/Volumes';
                        if (fs.existsSync(volumesDir)) {
                            const volumes = fs.readdirSync(volumesDir);
                            for (const volume of volumes) {
                                const mountPoint = path.join(volumesDir, volume);
                                try {
                                    const stats = fs.statSync(mountPoint);
                                    if (stats.isDirectory()) {
                                        const usage = await diskusage.check(mountPoint);
                                        const device = {
                                            id: `manual-${volume.replace(/[^a-zA-Z0-9]/g, '-')}`,
                                            name: volume,
                                            mountPoint: mountPoint,
                                            type: 'external',
                                            isRemovable: true,
                                            capacity: usage.total,
                                            free: usage.free,
                                            usedPct: Math.round(((usage.total - usage.free) / usage.total) * 100)
                                        };
                                        devices.push(device);
                                    }
                                }
                                catch (error) {
                                    console.warn(`Failed to process volume ${volume}:`, error);
                                }
                            }
                        }
                    }
                    catch (volumeError) {
                        console.warn('Failed to scan /Volumes directory:', volumeError);
                    }
                }
            }
        }
        // Add "Choose folder..." option
        devices.push({
            id: 'choose-folder',
            name: 'Choose folder...',
            mountPoint: '',
            type: 'internal',
            isRemovable: false,
            capacity: 0,
            free: 0,
            usedPct: 0
        });
        return { ok: true, data: devices };
    }
    catch (error) {
        console.error('Failed to list devices:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Choose custom folder
electron_1.ipcMain.handle('vault:chooseFolder', async () => {
    try {
        const result = await electron_1.dialog.showOpenDialog({
            properties: ['openDirectory', 'createDirectory'],
            title: 'Choose FamilyVault Storage Location'
        });
        if (result.canceled || result.filePaths.length === 0) {
            return { ok: false, error: 'No folder selected' };
        }
        return { ok: true, data: result.filePaths[0] };
    }
    catch (error) {
        console.error('Failed to choose folder:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Set vault selection
electron_1.ipcMain.handle('vault:setSelection', async (_, groupId, userId, mountPoint) => {
    try {
        // Create the vault path
        const vaultPath = path.join(mountPoint, 'FamilyVault', groupId, userId);
        // Ensure directory exists
        fs.mkdirSync(vaultPath, { recursive: true });
        // Update config
        const newConfig = { ...config };
        if (!newConfig.storage[groupId]) {
            newConfig.storage[groupId] = {};
        }
        newConfig.storage[groupId][userId] = {
            mountPoint,
            absolutePath: vaultPath,
            quotaBytes: 10 * 1024 * 1024 * 1024 // 10GB default quota
        };
        newConfig.currentGroupId = groupId;
        newConfig.userId = userId;
        saveConfig(newConfig);
        // Create manifest if it doesn't exist
        const manifestPath = path.join(vaultPath, '.vault.manifest.json');
        if (!fs.existsSync(manifestPath)) {
            const manifest = {
                groupId,
                userId,
                createdAt: new Date().toISOString(),
                files: []
            };
            fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
        }
        return { ok: true, data: vaultPath };
    }
    catch (error) {
        console.error('Failed to set vault selection:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Update quota
electron_1.ipcMain.handle('vault:updateQuota', async (_, groupId, userId, quotaBytes) => {
    try {
        const newConfig = { ...config };
        if (!newConfig.storage[groupId]?.[userId]) {
            return { ok: false, error: 'No vault assignment found' };
        }
        newConfig.storage[groupId][userId].quotaBytes = quotaBytes;
        saveConfig(newConfig);
        return { ok: true };
    }
    catch (error) {
        console.error('Failed to update quota:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Get vault assignment
electron_1.ipcMain.handle('vault:getAssignment', async (_, groupId, userId) => {
    try {
        const assignment = config.storage[groupId]?.[userId];
        if (!assignment) {
            return { ok: false, error: 'No vault assignment found' };
        }
        // Get current usage
        let currentSize = 0;
        try {
            const usage = await diskusage.check(assignment.absolutePath);
            const manifestPath = path.join(assignment.absolutePath, '.vault.manifest.json');
            if (fs.existsSync(manifestPath)) {
                const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
                currentSize = manifest.files.reduce((sum, file) => sum + file.size, 0);
            }
        }
        catch (error) {
            console.warn('Failed to get current usage:', error);
        }
        return {
            ok: true,
            data: {
                ...assignment,
                currentSize,
                freeSpace: (assignment.quotaBytes || 0) - currentSize
            }
        };
    }
    catch (error) {
        console.error('Failed to get vault assignment:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Copy files with progress
electron_1.ipcMain.handle('vault:copyFiles', async (event, groupId, userId, filePaths) => {
    try {
        const assignment = config.storage[groupId]?.[userId];
        if (!assignment) {
            return { ok: false, error: 'No vault assignment found' };
        }
        const manifestPath = path.join(assignment.absolutePath, '.vault.manifest.json');
        let manifest;
        try {
            manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
        }
        catch (error) {
            manifest = {
                groupId,
                userId,
                createdAt: new Date().toISOString(),
                files: []
            };
        }
        // Check quota
        const currentSize = manifest.files.reduce((sum, file) => sum + file.size, 0);
        let totalNewSize = 0;
        for (const filePath of filePaths) {
            const stats = fs.statSync(filePath);
            totalNewSize += stats.size;
        }
        if (assignment.quotaBytes && (currentSize + totalNewSize) > assignment.quotaBytes) {
            return { ok: false, error: 'Quota exceeded' };
        }
        // Copy files
        for (let i = 0; i < filePaths.length; i++) {
            const filePath = filePaths[i];
            const fileName = path.basename(filePath);
            const fileId = crypto.randomUUID();
            const destPath = path.join(assignment.absolutePath, fileId);
            // Send progress
            event.sender.send('vault:copyProgress', {
                current: i + 1,
                total: filePaths.length,
                fileName,
                fileId
            });
            // Copy file
            await fs.promises.copyFile(filePath, destPath);
            // Calculate hash
            const hash = crypto.createHash('sha256');
            const stream = fs.createReadStream(destPath);
            for await (const chunk of stream) {
                hash.update(chunk);
            }
            // Update manifest
            const stats = fs.statSync(destPath);
            manifest.files.push({
                id: fileId,
                name: fileName,
                size: stats.size,
                hash: hash.digest('hex'),
                createdAt: new Date().toISOString()
            });
        }
        // Save manifest
        fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
        return { ok: true };
    }
    catch (error) {
        console.error('Failed to copy files:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Open in Finder
electron_1.ipcMain.handle('vault:openInFinder', async (_, groupId, userId) => {
    try {
        const assignment = config.storage[groupId]?.[userId];
        if (!assignment) {
            return { ok: false, error: 'No vault assignment found' };
        }
        electron_1.shell.showItemInFolder(assignment.absolutePath);
        return { ok: true };
    }
    catch (error) {
        console.error('Failed to open in Finder:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// List files in vault
electron_1.ipcMain.handle('vault:listFiles', async (_, groupId, userId) => {
    try {
        const assignment = config.storage[groupId]?.[userId];
        if (!assignment) {
            return { ok: false, error: 'No vault assignment found' };
        }
        const manifestPath = path.join(assignment.absolutePath, '.vault.manifest.json');
        if (!fs.existsSync(manifestPath)) {
            return { ok: true, data: [] };
        }
        const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
        return { ok: true, data: manifest.files };
    }
    catch (error) {
        console.error('Failed to list files:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
// Download file from vault
electron_1.ipcMain.handle('vault:downloadFile', async (_, groupId, userId, fileId) => {
    try {
        const assignment = config.storage[groupId]?.[userId];
        if (!assignment) {
            return { ok: false, error: 'No vault assignment found' };
        }
        const manifestPath = path.join(assignment.absolutePath, '.vault.manifest.json');
        if (!fs.existsSync(manifestPath)) {
            return { ok: false, error: 'No files found' };
        }
        const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
        const fileInfo = manifest.files.find(f => f.id === fileId);
        if (!fileInfo) {
            return { ok: false, error: 'File not found' };
        }
        const sourcePath = path.join(assignment.absolutePath, fileId);
        if (!fs.existsSync(sourcePath)) {
            return { ok: false, error: 'File data not found' };
        }
        // Show save dialog
        const result = await electron_1.dialog.showSaveDialog({
            defaultPath: fileInfo.name,
            title: 'Save file as...'
        });
        if (result.canceled || !result.filePath) {
            return { ok: false, error: 'Download cancelled' };
        }
        // Copy file to chosen location
        await fs.promises.copyFile(sourcePath, result.filePath);
        return { ok: true };
    }
    catch (error) {
        console.error('Failed to download file:', error);
        return { ok: false, error: error instanceof Error ? error.message : 'Unknown error' };
    }
});
//# sourceMappingURL=vault.js.map