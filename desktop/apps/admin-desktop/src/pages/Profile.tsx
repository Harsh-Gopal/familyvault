import { useQuery } from '@tanstack/react-query';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { User, Mail, Phone, Calendar, Shield, LogOut } from 'lucide-react';
import Navigation from '../components/Navigation';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';
import { signOut } from '../lib/auth';
import { formatDateTime, getRoleColor, formatBytes } from '../lib/utils';

export default function Profile() {
  const { user } = useAuthStore();

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

  const handleSignOut = async () => {
    await signOut();
  };

  const myUsage = usage?.users.find(u => u.user_id === user?.claims.user_id);

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto">
        <div className="p-6">
          <div className="mb-6">
            <h1 className="text-2xl font-bold text-foreground">Profile</h1>
            <p className="text-muted-foreground">
              Your account information and preferences
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* User Information */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <User className="mr-2 h-5 w-5" />
                  User Information
                </CardTitle>
                <CardDescription>
                  Your personal details and contact information
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center space-x-4">
                  <div className="w-16 h-16 bg-primary rounded-full flex items-center justify-center">
                    <span className="text-xl font-medium text-primary-foreground">
                      {user?.user.display_name.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div>
                    <h3 className="text-lg font-medium">{user?.user.display_name}</h3>
                    <div className="flex items-center space-x-2">
                      <span className={`px-2 py-1 text-xs rounded-full ${getRoleColor(user?.claims.role || '')}`}>
                        {user?.claims.role}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="space-y-3">
                  {user?.user.email && (
                    <div className="flex items-center space-x-3">
                      <Mail className="h-4 w-4 text-muted-foreground" />
                      <span className="text-sm">{user.user.email}</span>
                    </div>
                  )}

                  {user?.user.phone && (
                    <div className="flex items-center space-x-3">
                      <Phone className="h-4 w-4 text-muted-foreground" />
                      <span className="text-sm">{user.user.phone}</span>
                    </div>
                  )}

                  <div className="flex items-center space-x-3">
                    <Calendar className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm">
                      Joined {formatDateTime(user?.user.created_at || '')}
                    </span>
                  </div>
                </div>

                <div className="pt-4 border-t">
                  <Button variant="outline" className="w-full">
                    Edit Profile
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Group Membership */}
            <Card>
              <CardHeader>
                <CardTitle>Group Membership</CardTitle>
                <CardDescription>
                  Your role and permissions in family groups
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="p-4 border rounded-lg">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="font-medium">{user?.group.name}</h4>
                    <span className={`px-2 py-1 text-xs rounded-full ${getRoleColor(user?.claims.role || '')}`}>
                      {user?.claims.role}
                    </span>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    Member since {formatDateTime(user?.membership.created_at || '')}
                  </p>
                </div>

                <div className="space-y-2">
                  <h5 className="font-medium text-sm">Permissions</h5>
                  <div className="space-y-1 text-sm text-muted-foreground">
                    {user?.claims.role === 'admin' && (
                      <>
                        <div className="flex items-center space-x-2">
                          <Shield className="h-3 w-3" />
                          <span>Manage group members</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Shield className="h-3 w-3" />
                          <span>Start/stop sessions</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Shield className="h-3 w-3" />
                          <span>Send notifications</span>
                        </div>
                      </>
                    )}
                    {(user?.claims.role === 'admin' || user?.claims.role === 'member') && (
                      <>
                        <div className="flex items-center space-x-2">
                          <Shield className="h-3 w-3" />
                          <span>Upload files</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Shield className="h-3 w-3" />
                          <span>Delete own files</span>
                        </div>
                      </>
                    )}
                    <div className="flex items-center space-x-2">
                      <Shield className="h-3 w-3" />
                      <span>Download files</span>
                    </div>
                    <div className="flex items-center space-x-2">
                      <Shield className="h-3 w-3" />
                      <span>View logs and metrics</span>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Usage Statistics */}
            {myUsage && (
              <Card>
                <CardHeader>
                  <CardTitle>Storage Usage</CardTitle>
                  <CardDescription>
                    Your current storage usage and limits
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">Used Storage</span>
                      <span className="text-sm text-muted-foreground">
                        {formatBytes(myUsage.used_bytes)}
                      </span>
                    </div>
                    
                    {myUsage.quota_bytes && (
                      <>
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium">Storage Quota</span>
                          <span className="text-sm text-muted-foreground">
                            {formatBytes(myUsage.quota_bytes)}
                          </span>
                        </div>
                        
                        <div className="w-full bg-secondary rounded-full h-2">
                          <div 
                            className="bg-primary h-2 rounded-full" 
                            style={{ 
                              width: `${Math.min((myUsage.used_bytes / myUsage.quota_bytes) * 100, 100)}%` 
                            }}
                          />
                        </div>
                        
                        <p className="text-xs text-muted-foreground">
                          {((myUsage.used_bytes / myUsage.quota_bytes) * 100).toFixed(1)}% of quota used
                        </p>
                      </>
                    )}
                  </div>

                  <div className="flex items-center justify-between pt-2 border-t">
                    <span className="text-sm font-medium">Files Uploaded</span>
                    <span className="text-sm text-muted-foreground">
                      {myUsage.file_count}
                    </span>
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Device Information */}
            <Card>
              <CardHeader>
                <CardTitle>Device Information</CardTitle>
                <CardDescription>
                  Information about this device and session
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-3">
                  <div>
                    <p className="text-sm font-medium">Device ID</p>
                    <p className="text-xs font-mono text-muted-foreground">
                      {user?.claims.device_id}
                    </p>
                  </div>

                  <div>
                    <p className="text-sm font-medium">Session Expires</p>
                    <p className="text-sm text-muted-foreground">
                      {new Date(user?.claims.exp ? user.claims.exp * 1000 : 0).toLocaleString()}
                    </p>
                  </div>

                  <div>
                    <p className="text-sm font-medium">Token Issued</p>
                    <p className="text-sm text-muted-foreground">
                      {new Date(user?.claims.iat ? user.claims.iat * 1000 : 0).toLocaleString()}
                    </p>
                  </div>
                </div>

                <div className="pt-4 border-t">
                  <Button 
                    variant="destructive" 
                    className="w-full"
                    onClick={handleSignOut}
                  >
                    <LogOut className="mr-2 h-4 w-4" />
                    Sign Out
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}