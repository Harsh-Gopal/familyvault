import { useState, useEffect } from 'react';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { 
  HardDrive, 
  ExternalLink, 
  Folder, 
  FolderOpen,
  Loader2,
  Upload,
  Settings,
  CheckCircle
} from 'lucide-react';
import { useAuthStore } from '../store/auth';
import { DeviceInfo } from '../../electron/preload';

interface VaultManagerProps {
  onClose?: () => void;
}

interface VaultAssignment {
  mountPoint: string;
  absolutePath: string;
  quotaBytes?: number;
  currentSize: number;
  freeSpace: number;
}

export default function VaultManager({ onClose }: VaultManagerProps) {
  const { user } = useAuthStore();
  const [devices, setDevices] = useState<DeviceInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedDevice, setSelectedDevice] = useState<DeviceInfo | null>(null);
  const [assignment, setAssignment] = useState<VaultAssignment | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<any>(null);
  const [files, setFiles] = useState<any[]>([]);
  const [downloading, setDownloading] = useState<string | null>(null);

  useEffect(() => {
    loadDevices();
    loadAssignment();
  }, []);

  useEffect(() => {
    // Listen for upload progress
    const handleProgress = (progress: any) => {
      setUploadProgress(progress);
    };

    window.fv.vault.onCopyProgress(handleProgress);

    return () => {
      window.fv.removeAllListeners('vault:copyProgress');
    };
  }, []);

  const loadDevices = async () => {
    try {
      const result = await window.fv.vault.listDevices();
      if (result.ok && result.data) {
        setDevices(result.data);
      }
    } catch (error) {
      console.error('Failed to load devices:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadAssignment = async () => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    try {
      const result = await window.fv.vault.getAssignment(user.claims.group_id, user.claims.user_id);
      if (result.ok && result.data) {
        setAssignment(result.data);
        await loadFiles();
      }
    } catch (error) {
      console.error('Failed to load assignment:', error);
    }
  };

  const loadFiles = async () => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    try {
      const result = await window.fv.vault.listFiles(user.claims.group_id, user.claims.user_id);
      if (result.ok && result.data) {
        setFiles(result.data);
      }
    } catch (error) {
      console.error('Failed to load files:', error);
    }
  };

  const handleDeviceSelect = async (device: DeviceInfo) => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    let mountPoint = device.mountPoint;

    // Handle "Choose folder..." option
    if (device.id === 'choose-folder') {
      const result = await window.fv.vault.chooseFolder();
      if (!result.ok || !result.data) return;
      mountPoint = result.data;
    }

    try {
      const result = await window.fv.vault.setSelection(
        user.claims.group_id,
        user.claims.user_id,
        mountPoint
      );

      if (result.ok) {
        await loadAssignment();
        setSelectedDevice(device);
      }
    } catch (error) {
      console.error('Failed to set device selection:', error);
    }
  };

  const handleUpload = async () => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    try {
      const filePaths = await window.fv.openFileDialog({
        properties: ['openFile', 'multiSelections'],
        title: 'Select files to upload'
      });

      if (filePaths.length === 0) return;

      setUploading(true);
      setUploadProgress(null);

      const result = await window.fv.vault.copyFiles(
        user.claims.group_id,
        user.claims.user_id,
        filePaths
      );

      if (result.ok) {
        await loadAssignment();
        await loadFiles();
      } else {
        alert(`Upload failed: ${result.error}`);
      }
    } catch (error) {
      console.error('Upload failed:', error);
      alert('Upload failed');
    } finally {
      setUploading(false);
      setUploadProgress(null);
    }
  };

  const handleOpenInFinder = async () => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    try {
      await window.fv.vault.openInFinder(user.claims.group_id, user.claims.user_id);
    } catch (error) {
      console.error('Failed to open in Finder:', error);
    }
  };

  const handleDownloadFile = async (fileId: string) => {
    if (!user?.claims.group_id || !user?.claims.user_id) return;

    setDownloading(fileId);
    try {
      const result = await window.fv.vault.downloadFile(user.claims.group_id, user.claims.user_id, fileId);
      if (!result.ok) {
        alert(`Download failed: ${result.error}`);
      }
    } catch (error) {
      console.error('Failed to download file:', error);
      alert('Download failed');
    } finally {
      setDownloading(null);
    }
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getDeviceIcon = (device: DeviceInfo) => {
    if (device.id === 'choose-folder') return <Folder className="h-5 w-5" />;
    if (device.type === 'external') return <ExternalLink className="h-5 w-5" />;
    return <HardDrive className="h-5 w-5" />;
  };

  const getUsageColor = (usedPct: number) => {
    if (usedPct >= 90) return 'bg-red-500';
    if (usedPct >= 75) return 'bg-yellow-500';
    return 'bg-green-500';
  };

  if (loading) {
    return (
      <Card className="w-full max-w-2xl">
        <CardContent className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin" />
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="w-full max-w-4xl space-y-6">
      {/* Current Assignment */}
      {assignment && (
        <Card className="bg-gradient-to-br from-green-50 to-emerald-100 dark:from-green-900/20 dark:to-emerald-900/20 border-green-200 dark:border-green-800">
          <CardHeader>
            <CardTitle className="flex items-center text-green-800 dark:text-green-200">
              <CheckCircle className="mr-2 h-5 w-5" />
              Vault Configured
            </CardTitle>
            <CardDescription className="text-green-700 dark:text-green-300">
              Your vault is ready for uploads
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <p className="text-sm font-medium text-green-800 dark:text-green-200">Location</p>
                <p className="text-sm text-green-700 dark:text-green-300 truncate">
                  {assignment.absolutePath}
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-green-800 dark:text-green-200">Used Space</p>
                <p className="text-sm text-green-700 dark:text-green-300">
                  {formatBytes(assignment.currentSize)}
                  {assignment.quotaBytes && ` of ${formatBytes(assignment.quotaBytes)}`}
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-green-800 dark:text-green-200">Available</p>
                <p className="text-sm text-green-700 dark:text-green-300">
                  {formatBytes(assignment.freeSpace)}
                </p>
              </div>
            </div>

            {assignment.quotaBytes && (
              <div>
                <div className="flex justify-between text-sm mb-1">
                  <span className="text-green-800 dark:text-green-200">Storage Usage</span>
                  <span className="text-green-700 dark:text-green-300">
                    {Math.round((assignment.currentSize / assignment.quotaBytes) * 100)}%
                  </span>
                </div>
                <div className="w-full bg-green-200 dark:bg-green-800 rounded-full h-2">
                  <div
                    className="bg-green-600 dark:bg-green-400 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${Math.min((assignment.currentSize / assignment.quotaBytes) * 100, 100)}%` }}
                  />
                </div>
              </div>
            )}

            <div className="flex space-x-2">
              <Button
                onClick={handleUpload}
                disabled={uploading}
                className="bg-green-600 hover:bg-green-700 text-white"
              >
                {uploading ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Upload className="mr-2 h-4 w-4" />
                )}
                {uploading ? 'Uploading...' : 'Upload Files'}
              </Button>
              <Button
                variant="outline"
                onClick={handleOpenInFinder}
                className="border-green-300 text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-300 dark:hover:bg-green-900/20"
              >
                <FolderOpen className="mr-2 h-4 w-4" />
                Open in Finder
              </Button>
              <Button
                variant="outline"
                onClick={() => setAssignment(null)}
                className="border-green-300 text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-300 dark:hover:bg-green-900/20"
              >
                <Settings className="mr-2 h-4 w-4" />
                Change
              </Button>
            </div>

            {/* Upload Progress */}
            {uploadProgress && (
              <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-green-200 dark:border-green-800">
                <div className="flex justify-between text-sm mb-1">
                  <span>Uploading {uploadProgress.fileName}</span>
                  <span>{uploadProgress.current} of {uploadProgress.total}</span>
                </div>
                <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                  <div
                    className="bg-green-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${(uploadProgress.current / uploadProgress.total) * 100}%` }}
                  />
                </div>
              </div>
            )}

            {/* File List */}
            {files.length > 0 && (
              <div className="mt-6">
                <h3 className="font-medium text-green-800 dark:text-green-200 mb-3">Uploaded Files ({files.length})</h3>
                <div className="space-y-2 max-h-48 overflow-y-auto">
                  {files.map((file) => (
                    <div
                      key={file.id}
                      className="flex items-center justify-between p-3 bg-white dark:bg-gray-800 rounded-lg border border-green-200 dark:border-green-800"
                    >
                      <div className="flex-1 min-w-0">
                        <p className="font-medium text-green-800 dark:text-green-200 truncate">
                          {file.name}
                        </p>
                        <p className="text-sm text-green-700 dark:text-green-300">
                          {formatBytes(file.size)} • {new Date(file.createdAt).toLocaleDateString()}
                        </p>
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleDownloadFile(file.id)}
                        disabled={downloading === file.id}
                        className="ml-3 border-green-300 text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-300 dark:hover:bg-green-900/20"
                      >
                        {downloading === file.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          'Download'
                        )}
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Device Selection */}
      {!assignment && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <HardDrive className="mr-2 h-5 w-5" />
              Choose Storage Location
            </CardTitle>
            <CardDescription>
              Select where to store your FamilyVault files
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {devices.map((device) => (
                <Card
                  key={device.id}
                  className={`cursor-pointer transition-all duration-200 hover:shadow-md ${
                    selectedDevice?.id === device.id
                      ? 'ring-2 ring-primary border-primary'
                      : 'hover:border-primary/50'
                  }`}
                  onClick={() => handleDeviceSelect(device)}
                >
                  <CardContent className="p-4">
                    <div className="flex items-start space-x-3">
                      <div className="flex-shrink-0 mt-1">
                        {getDeviceIcon(device)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <h3 className="font-medium text-foreground truncate">
                          {device.name}
                        </h3>
                        {device.mountPoint && (
                          <p className="text-sm text-muted-foreground truncate">
                            {device.mountPoint}
                          </p>
                        )}
                        <div className="flex items-center space-x-2 mt-1">
                          <span className={`px-2 py-1 text-xs rounded-full ${
                            device.type === 'external' 
                              ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                              : device.type === 'network'
                              ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                              : 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200'
                          }`}>
                            {device.type}
                          </span>
                          {device.isRemovable && (
                            <span className="px-2 py-1 text-xs rounded-full bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200">
                              removable
                            </span>
                          )}
                        </div>
                        
                        {device.capacity > 0 && (
                          <div className="mt-3">
                            <div className="flex justify-between text-xs mb-1">
                              <span className="text-muted-foreground">
                                {formatBytes(device.capacity - device.free)} used
                              </span>
                              <span className="text-muted-foreground">
                                {device.usedPct}%
                              </span>
                            </div>
                            <div className="w-full bg-muted rounded-full h-1.5">
                              <div
                                className={`h-1.5 rounded-full transition-all duration-300 ${getUsageColor(device.usedPct)}`}
                                style={{ width: `${device.usedPct}%` }}
                              />
                            </div>
                            <p className="text-xs text-muted-foreground mt-1">
                              {formatBytes(device.free)} available of {formatBytes(device.capacity)}
                            </p>
                          </div>
                        )}
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {onClose && (
        <div className="flex justify-end">
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </div>
      )}
    </div>
  );
}