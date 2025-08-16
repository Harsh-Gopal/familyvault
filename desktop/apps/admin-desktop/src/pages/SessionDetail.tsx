import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, CardContent, CardHeader, CardTitle } from '@familyvault/ui';
import { 
  ArrowLeft, 
  Upload, 
  Download, 
  FileText, 
  Activity, 
  BarChart3,
  Trash2,
  Eye
} from 'lucide-react';
import Navigation from '../components/Navigation';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';
import { formatDateTime, formatBytes, getRoleColor } from '../lib/utils';

const tabs = [
  { id: 'files', name: 'Files', icon: FileText },
  { id: 'logs', name: 'Logs', icon: Activity },
  { id: 'metrics', name: 'Metrics', icon: BarChart3 },
  { id: 'status', name: 'Status', icon: Eye },
];

export default function SessionDetail() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState('files');

  const isAdmin = user?.claims.role === 'admin';
  const canUpload = user?.claims.role === 'admin' || user?.claims.role === 'member';

  // Session query
  const { data: session } = useQuery({
    queryKey: ['session', user?.claims.group_id, sessionId],
    queryFn: async () => {
      if (!user?.claims.group_id || !sessionId) return null;
      const api = await getAPI();
      return api.getSession(user.claims.group_id, sessionId);
    },
    enabled: !!user?.claims.group_id && !!sessionId,
  });

  // Session status query
  const { data: status } = useQuery({
    queryKey: ['session-status', user?.claims.group_id, sessionId],
    queryFn: async () => {
      if (!user?.claims.group_id || !sessionId) return null;
      const api = await getAPI();
      return api.getSessionStatus(user.claims.group_id, sessionId);
    },
    enabled: !!user?.claims.group_id && !!sessionId,
    refetchInterval: 30000,
  });

  // Files query
  const { data: files } = useQuery({
    queryKey: ['session-files', user?.claims.group_id, sessionId],
    queryFn: async () => {
      if (!user?.claims.group_id || !sessionId) return null;
      const api = await getAPI();
      return api.listFiles(user.claims.group_id, sessionId);
    },
    enabled: !!user?.claims.group_id && !!sessionId && activeTab === 'files',
  });

  // Logs query
  const { data: logs } = useQuery({
    queryKey: ['session-logs', user?.claims.group_id, sessionId],
    queryFn: async () => {
      if (!user?.claims.group_id || !sessionId) return null;
      const api = await getAPI();
      return api.getLogs(user.claims.group_id, sessionId, { limit: 100 });
    },
    enabled: !!user?.claims.group_id && !!sessionId && activeTab === 'logs',
  });

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files || !user?.claims.group_id || !sessionId) return;

    for (const file of Array.from(files)) {
      try {
        const api = await getAPI();
        await api.uploadFile(user.claims.group_id, sessionId, file);
      } catch (error) {
        console.error('Failed to upload file:', error);
      }
    }

    // Reset input
    event.target.value = '';
  };

  const handleDownloadFile = async (filename: string) => {
    if (!user?.claims.group_id || !sessionId) return;

    try {
      const api = await getAPI();
      const blob = await api.downloadFile(user.claims.group_id, sessionId, filename);
      
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (error) {
      console.error('Failed to download file:', error);
    }
  };

  const handleDeleteFile = async (filename: string) => {
    if (!user?.claims.group_id || !sessionId) return;

    try {
      const api = await getAPI();
      await api.deleteFile(user.claims.group_id, sessionId, filename);
    } catch (error) {
      console.error('Failed to delete file:', error);
    }
  };

  const renderTabContent = () => {
    switch (activeTab) {
      case 'files':
        return (
          <div className="space-y-4">
            {canUpload && (
              <div className="border-2 border-dashed border-border rounded-lg p-6">
                <div className="text-center">
                  <Upload className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-sm text-muted-foreground mb-4">
                    Drag and drop files here, or click to select
                  </p>
                  <input
                    type="file"
                    multiple
                    onChange={handleFileUpload}
                    className="hidden"
                    id="file-upload"
                  />
                  <Button asChild>
                    <label htmlFor="file-upload" className="cursor-pointer">
                      Select Files
                    </label>
                  </Button>
                </div>
              </div>
            )}

            {files && files.files.length > 0 ? (
              <div className="space-y-2">
                {files.files.map((file) => (
                  <div
                    key={file.filename}
                    className="flex items-center justify-between p-3 border rounded-lg"
                  >
                    <div className="flex-1">
                      <p className="font-medium">{file.filename}</p>
                      <p className="text-sm text-muted-foreground">
                        {formatBytes(file.size)} • {formatDateTime(file.modified_time)}
                      </p>
                    </div>
                    
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleDownloadFile(file.filename)}
                      >
                        <Download className="h-4 w-4" />
                      </Button>
                      
                      {(isAdmin || canUpload) && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleDeleteFile(file.filename)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <FileText className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <p className="text-muted-foreground">No files in this session</p>
              </div>
            )}
          </div>
        );

      case 'logs':
        return (
          <div className="space-y-2">
            {logs && logs.logs.length > 0 ? (
              logs.logs.map((log, index) => (
                <div key={index} className="p-3 border rounded-lg font-mono text-sm">
                  <div className="flex items-center space-x-2">
                    <span className="text-muted-foreground">
                      {formatDateTime(log.timestamp)}
                    </span>
                    <span className={`px-2 py-1 text-xs rounded ${getRoleColor(log.level)}`}>
                      {log.level.toUpperCase()}
                    </span>
                    <span>{log.message}</span>
                  </div>
                </div>
              ))
            ) : (
              <div className="text-center py-8">
                <Activity className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <p className="text-muted-foreground">No logs available</p>
              </div>
            )}
          </div>
        );

      case 'metrics':
        return (
          <div className="text-center py-8">
            <BarChart3 className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
            <p className="text-muted-foreground">Metrics coming soon</p>
          </div>
        );

      case 'status':
        return (
          <div className="space-y-4">
            {status && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">Session Status</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">
                      {status.is_active ? 'Active' : 'Expired'}
                    </p>
                    <p className="text-sm text-muted-foreground">
                      {status.remaining_seconds > 0 
                        ? `${Math.floor(status.remaining_seconds / 60)} minutes remaining`
                        : 'Session has expired'
                      }
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">File Count</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">
                      {files?.total_count || 0}
                    </p>
                    <p className="text-sm text-muted-foreground">
                      Files uploaded
                    </p>
                  </CardContent>
                </Card>
              </div>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  if (!session) {
    return (
      <div className="flex h-screen bg-background">
        <Navigation />
        <main className="flex-1 flex items-center justify-center">
          <p className="text-muted-foreground">Session not found</p>
        </main>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto">
        <div className="p-6">
          <div className="flex items-center space-x-4 mb-6">
            <Button variant="ghost" size="sm" asChild>
              <Link to="/sessions">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div>
              <h1 className="text-2xl font-bold text-foreground">Session Details</h1>
              <p className="text-muted-foreground font-mono text-sm">
                {session.session_id}
              </p>
            </div>
          </div>

          {/* Tab Navigation */}
          <div className="border-b border-border mb-6">
            <nav className="flex space-x-8">
              {tabs.map((tab) => {
                const Icon = tab.icon;
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={`flex items-center space-x-2 py-2 px-1 border-b-2 font-medium text-sm ${
                      activeTab === tab.id
                        ? 'border-primary text-primary'
                        : 'border-transparent text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    <Icon className="h-4 w-4" />
                    <span>{tab.name}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          {/* Tab Content */}
          {renderTabContent()}
        </div>
      </main>
    </div>
  );
}