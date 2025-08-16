"use strict";
// Backend process management utilities
// This file contains utilities for managing the backend process lifecycle
Object.defineProperty(exports, "__esModule", { value: true });
exports.BackendManager = void 0;
class BackendManager {
    static instance;
    process = null;
    port = 8000;
    static getInstance() {
        if (!BackendManager.instance) {
            BackendManager.instance = new BackendManager();
        }
        return BackendManager.instance;
    }
    getStatus() {
        return {
            running: this.process !== null && !this.process.killed,
            port: this.port,
            pid: this.process?.pid,
        };
    }
    async stop() {
        if (this.process) {
            this.process.kill('SIGTERM');
            this.process = null;
        }
    }
}
exports.BackendManager = BackendManager;
//# sourceMappingURL=backend.js.map