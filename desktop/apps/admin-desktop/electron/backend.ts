// Backend process management utilities
// This file contains utilities for managing the backend process lifecycle

export interface BackendStatus {
  running: boolean;
  port: number;
  pid?: number;
}

export class BackendManager {
  private static instance: BackendManager;
  private process: any = null;
  private port = 8000;

  static getInstance(): BackendManager {
    if (!BackendManager.instance) {
      BackendManager.instance = new BackendManager();
    }
    return BackendManager.instance;
  }

  getStatus(): BackendStatus {
    return {
      running: this.process !== null && !this.process.killed,
      port: this.port,
      pid: this.process?.pid,
    };
  }

  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM');
      this.process = null;
    }
  }
}