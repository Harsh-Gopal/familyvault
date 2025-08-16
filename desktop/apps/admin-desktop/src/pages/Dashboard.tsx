import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { 
  HardDrive, 
  Users, 
  Bell, 
  Activity,
  UserPlus
} from 'lucide-react';
import Navigation from '../components/Navigation';
import SessionControl from '../components/SessionControl';
import { useAuthStore } from '../store/auth';
import { useSessionStore } from '../store/session';
import { getAPI } from '../lib/api';
import { formatBytes } from '../lib/utils';

export default function Dashboard() {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const { activeSession, setActiveSession } = useSessionStore();
  const [showNotificationModal, setShowNotificationModal] = useState(false);

  const isAdmin = user?.claims.role === 'admin';

  // Health check query
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const api = await getAPI();
      return api.health();
    },
    refetchInterval: 30000, // Refresh every 30 seconds
  });

  // Active session query
  const { data: sessions, refetch: refetchSessions } = useQuery({
    queryKey: ['active-sessions', user?.claims.group_id],
    queryFn: async () => {
      if (!user?.claims.group_id) return [];
      const api = await getAPI();
      return api.getActiveSession(user.claims.group_id);
    },
    refetchInterval: 10000, // Refresh every 10 seconds
  });

  // Usage query
  const { data: usage } = useQuery({
    queryKey: ['usage', user?.claims.group_id],
    queryFn: async () => {
      if (!user?.claims.group_id) return null;
      const api = await getAPI();
      return api.getUsage(user.claims.group_id);
    },
    enabled: !!user?.claims.group_id,
  });

  useEffect(() => {
    if (sessions && sessions.length > 0) {
      setActiveSession(sessions[0]);
    } else {
      setActiveSession(null);
    }
  }, [sessions, setActiveSession]);

  const handleSessionChange = () => {
    refetchSessions();
  };

  const myUsage = usage?.users.find(u => u.user_id === user?.claims.user_id);

  const handleManageMembers = () => {
    navigate('/members');
  };

  const handleSendNotification = () => {
    setShowNotificationModal(true);
  };

  const handleNotificationSend = (message: string) => {
    // Create mailto link with all group members
    const subject = encodeURIComponent('FamilyVault Notification');
    const body = encodeURIComponent(message);
    const mailtoLink = `mailto:?subject=${subject}&body=${body}`;
    
    // Open default email client
    window.open(mailtoLink);
    setShowNotificationModal(false);
  };

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto pt-8">
        <div className="p-6">
          <div className="mb-6 flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-foreground">Dashboard</h1>
              <p className="text-muted-foreground">
                Welcome back, {user?.user.display_name}
              </p>
            </div>
            <SessionControl onSessionChange={handleSessionChange} />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-6">
            {/* Vault Status */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Storage</CardTitle>
                <HardDrive className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-foreground">
                  {health?.status === 'ok' ? 'Online' : 'Offline'}
                </div>
                <div className="mt-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => navigate('/vault')}
                    className="text-xs"
                  >
                    Manage Vault
                  </Button>
                </div>
                {health?.drive_free_bytes && health?.drive_total_bytes && (
                  <p className="text-xs text-muted-foreground mt-1">
                    {formatBytes(health.drive_free_bytes)} free of {formatBytes(health.drive_total_bytes)}
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Active Session */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Active Session</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-foreground">
                  {activeSession ? 'Running' : 'None'}
                </div>
                {activeSession && (
                  <p className="text-xs text-muted-foreground">
                    Started {new Date(activeSession.created_at).toLocaleTimeString()}
                  </p>
                )}
              </CardContent>
            </Card>

            {/* My Usage */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">My Usage</CardTitle>
                <Users className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-foreground">
                  {myUsage ? formatBytes(myUsage.used_bytes) : '0 B'}
                </div>
                {myUsage?.quota_bytes && (
                  <p className="text-xs text-muted-foreground">
                    of {formatBytes(myUsage.quota_bytes)} quota
                  </p>
                )}
              </CardContent>
            </Card>
          </div>



          {/* Quick Actions */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Recent Activity</CardTitle>
                <CardDescription>
                  Latest files and session activity
                </CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  No recent activity
                </p>
              </CardContent>
            </Card>

            {isAdmin && (
              <Card>
                <CardHeader>
                  <CardTitle>Admin Actions</CardTitle>
                  <CardDescription>
                    Quick access to admin functions
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  <Button 
                    variant="outline" 
                    size="sm" 
                    className="w-full justify-start h-10 flex items-center"
                    onClick={() => navigate('/members?invite=true')}
                  >
                    <UserPlus className="mr-2 h-4 w-4 flex-shrink-0" />
                    <span>Add Member</span>
                  </Button>
                  <Button 
                    variant="outline" 
                    size="sm" 
                    className="w-full justify-start h-10 flex items-center"
                    onClick={handleManageMembers}
                  >
                    <Users className="mr-2 h-4 w-4 flex-shrink-0" />
                    <span>Manage Members</span>
                  </Button>
                  <Button 
                    variant="outline" 
                    size="sm" 
                    className="w-full justify-start h-10 flex items-center"
                    onClick={handleSendNotification}
                  >
                    <Bell className="mr-2 h-4 w-4 flex-shrink-0" />
                    <span>Send Notification</span>
                  </Button>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </main>

      {/* Notification Modal */}
      {showNotificationModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md mx-4">
            <CardHeader>
              <CardTitle>Send Notification</CardTitle>
              <CardDescription>
                Choose a quick message to send to all group members via email
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <h4 className="text-sm font-medium">Quick Messages</h4>
                <div className="space-y-2">
                  <Button
                    variant="outline"
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleNotificationSend("The family vault is now online and ready for uploads!")}
                  >
                    <div>
                      <div className="font-medium">Vault Online</div>
                      <div className="text-xs text-muted-foreground">Notify when the vault comes online</div>
                    </div>
                  </Button>
                  <Button
                    variant="outline"
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleNotificationSend("A new session has started. You can now upload your files!")}
                  >
                    <div>
                      <div className="font-medium">Session Started</div>
                      <div className="text-xs text-muted-foreground">Notify when a new session begins</div>
                    </div>
                  </Button>
                  <Button
                    variant="outline"
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleNotificationSend("The current session will end soon. Please complete your uploads.")}
                  >
                    <div>
                      <div className="font-medium">Session Ending</div>
                      <div className="text-xs text-muted-foreground">Warn about session expiration</div>
                    </div>
                  </Button>
                </div>
              </div>
              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setShowNotificationModal(false)}>
                  Cancel
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}


    </div>
  );
}