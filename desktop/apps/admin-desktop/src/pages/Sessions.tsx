import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { 
  Play, 
  Square, 
  FolderOpen, 
  Download, 
  Trash2,
  Loader2
} from 'lucide-react';
import Navigation from '../components/Navigation';
import { useAuthStore } from '../store/auth';
import { useSessionStore } from '../store/session';
import { getAPI } from '../lib/api';
import { formatDateTime, formatRelativeTime } from '../lib/utils';

export default function Sessions() {
  const { user } = useAuthStore();
  const { activeSession, setActiveSession } = useSessionStore();
  const [isStartingSession, setIsStartingSession] = useState(false);
  const [isStoppingSession, setIsStoppingSession] = useState(false);

  const isAdmin = user?.claims.role === 'admin';

  // Active session query
  const { data: sessions, refetch: refetchSessions } = useQuery({
    queryKey: ['active-sessions', user?.claims.group_id],
    queryFn: async () => {
      if (!user?.claims.group_id) return [];
      const api = await getAPI();
      return api.getActiveSession(user.claims.group_id);
    },
    refetchInterval: 10000,
  });

  const handleStartSession = async () => {
    if (!user?.claims.group_id) return;
    
    setIsStartingSession(true);
    try {
      const api = await getAPI();
      const session = await api.openSession(user.claims.group_id);
      setActiveSession(session);
      refetchSessions();
    } catch (error) {
      console.error('Failed to start session:', error);
    } finally {
      setIsStartingSession(false);
    }
  };

  const handleStopSession = async () => {
    if (!user?.claims.group_id) return;
    
    setIsStoppingSession(true);
    try {
      const api = await getAPI();
      await api.closeSession(user.claims.group_id);
      setActiveSession(null);
      refetchSessions();
    } catch (error) {
      console.error('Failed to stop session:', error);
    } finally {
      setIsStoppingSession(false);
    }
  };

  const handleDownloadAll = async (sessionId: string) => {
    if (!user?.claims.group_id) return;
    
    try {
      const api = await getAPI();
      const blob = await api.downloadAll(user.claims.group_id, sessionId);
      
      // Create download link
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `session-${sessionId}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (error) {
      console.error('Failed to download session:', error);
    }
  };

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto pt-8">
        <div className="p-6">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold text-foreground">Sessions</h1>
              <p className="text-muted-foreground">
                Manage upload sessions and access files
              </p>
            </div>
            
            {isAdmin && (
              <div className="flex space-x-2">
                {!activeSession ? (
                  <Button 
                    onClick={handleStartSession}
                    disabled={isStartingSession}
                    className="h-11 px-6 bg-gradient-to-r from-gray-500 to-gray-600 hover:from-gray-600 hover:to-gray-700 text-white font-medium rounded-lg shadow-md hover:shadow-lg transition-all duration-300 transform hover:scale-105"
                  >
                    {isStartingSession ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <Play className="mr-2 h-4 w-4" />
                    )}
                    {isStartingSession ? 'Starting...' : 'Start Session'}
                  </Button>
                ) : (
                  <Button 
                    onClick={handleStopSession}
                    disabled={isStoppingSession}
                    className="h-11 px-6 bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700 text-white font-medium rounded-lg shadow-md hover:shadow-lg transition-all duration-300 transform hover:scale-105"
                  >
                    {isStoppingSession ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <Square className="mr-2 h-4 w-4" />
                    )}
                    {isStoppingSession ? 'Stopping...' : 'Stop Session'}
                  </Button>
                )}
              </div>
            )}
          </div>

          {/* Active Session */}
          {activeSession && (
            <Card className="mb-6 border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-green-800 dark:text-green-200">
                      Active Session
                    </CardTitle>
                    <CardDescription className="text-green-600 dark:text-green-400">
                      Started {formatRelativeTime(activeSession.created_at)}
                    </CardDescription>
                  </div>
                  <div className="flex space-x-2">
                    <Button variant="outline" size="sm" asChild className="h-9 flex items-center">
                      <Link to={`/sessions/${activeSession.session_id}`}>
                        <FolderOpen className="mr-2 h-4 w-4 flex-shrink-0" />
                        <span>Open</span>
                      </Link>
                    </Button>
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => handleDownloadAll(activeSession.session_id)}
                      className="h-9 flex items-center"
                    >
                      <Download className="mr-2 h-4 w-4 flex-shrink-0" />
                      <span>Download All</span>
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Session</p>
                    <p className="font-medium">Active Upload Session</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Started By</p>
                    <p>{activeSession.started_by_user === user?.claims.user_id ? 'You' : 'Admin'}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Created</p>
                    <p>{formatDateTime(activeSession.created_at)}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Expires</p>
                    <p>{formatDateTime(activeSession.expires)}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Session History */}
          <Card>
            <CardHeader>
              <CardTitle>Session History</CardTitle>
              <CardDescription>
                Previous upload sessions and their files
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!sessions || sessions.length === 0 ? (
                <div className="text-center py-8">
                  <FolderOpen className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">No sessions found</p>
                  {isAdmin && (
                    <p className="text-sm text-muted-foreground mt-2">
                      Start a session to begin uploading files
                    </p>
                  )}
                </div>
              ) : (
                <div className="grid gap-4">
                  {sessions.map((session) => (
                    <Card key={session.session_id} className="hover:shadow-md transition-shadow">
                      <CardContent className="p-6">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center space-x-4">
                            <div className="w-3 h-3 rounded-full bg-green-500 flex-shrink-0" />
                            <div className="min-w-0 flex-1">
                              <h3 className="font-semibold text-lg">Upload Session</h3>
                              <p className="text-sm text-muted-foreground mt-1">
                                Started by {session.started_by_user === user?.claims.user_id ? 'You' : 'Admin'} • {formatDateTime(session.created_at)}
                              </p>
                              <div className="flex items-center space-x-4 mt-2 text-sm text-muted-foreground">
                                <span>Status: <span className="capitalize font-medium text-green-600">Active</span></span>
                                <span>•</span>
                                <span>Expires: {formatRelativeTime(session.expires)}</span>
                              </div>
                            </div>
                          </div>
                          
                          <div className="flex items-center space-x-2 flex-shrink-0">
                            <Button variant="outline" size="sm" asChild className="h-9 flex items-center">
                              <Link to={`/sessions/${session.session_id}`}>
                                <FolderOpen className="mr-2 h-4 w-4 flex-shrink-0" />
                                <span>View Details</span>
                              </Link>
                            </Button>
                            
                            <Button 
                              variant="outline" 
                              size="sm"
                              onClick={() => handleDownloadAll(session.session_id)}
                              className="h-9 flex items-center"
                            >
                              <Download className="mr-2 h-4 w-4 flex-shrink-0" />
                              <span>Download</span>
                            </Button>

                            {isAdmin && (
                              <Button variant="outline" size="sm" className="h-9 flex items-center">
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            )}
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}