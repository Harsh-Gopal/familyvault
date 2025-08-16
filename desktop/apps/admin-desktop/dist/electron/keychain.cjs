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
const keytar = __importStar(require("keytar"));
const SERVICE_NAME = 'FamilyVault';
const ACCOUNT_NAME = 'auth-token';
// Token management
electron_1.ipcMain.handle('set-token', async (_, token) => {
    try {
        await keytar.setPassword(SERVICE_NAME, ACCOUNT_NAME, token);
        return true;
    }
    catch (error) {
        console.error('Failed to store token in keychain:', error);
        throw error;
    }
});
electron_1.ipcMain.handle('get-token', async () => {
    try {
        return await keytar.getPassword(SERVICE_NAME, ACCOUNT_NAME);
    }
    catch (error) {
        console.error('Failed to retrieve token from keychain:', error);
        return null;
    }
});
electron_1.ipcMain.handle('clear-token', async () => {
    try {
        return await keytar.deletePassword(SERVICE_NAME, ACCOUNT_NAME);
    }
    catch (error) {
        console.error('Failed to clear token from keychain:', error);
        throw error;
    }
});
//# sourceMappingURL=keychain.js.map