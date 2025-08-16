import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { 
  Settings as SettingsIcon, 
  HardDrive, 
  Network, 
  Shield, 
  Trash2,
  ExternalLink,
  Moon,
  Sun,
  Monitor
} from 'lucide-react';
import Navigation from '../components/Navigation';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';
import { formatBytes } from '../lib/utils';

export default function Settings() {
  const { user } = useAuthStore();
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>('system');

  const isAdmin = user?.claims.role === 'admin';

  // Health query for storage info
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const api = await getAPI();
      return api.health();
    },
  });

  const handleOpenVaultFolder = async () => {
    if (health?.drive_path) {
      window.fv.showItemInFolder(health.drive_path);
    }
  };

  const handleThemeChange = (newTheme: 'light' | 'dark' | 'system') => {
    setTheme(newTheme);
    
    // Apply theme
    const root = window.document.documentElement;
    if (newTheme === 'system') {
      const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
      root.classList.toggle('dark', systemTheme === 'dark');
    } else {
      root.classList.toggle('dark', newTheme === 'dark');
    }
    
    // Store preference
    localStorage.setItem('theme', newTheme);
  };

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto">
        <div className="p-6">
          <div className="mb-6">
            <h1 className="text-2xl font-bold text-foreground">Settings</h1>
            <p className="text-muted-foreground">
              Manage your vault and application preferences
            </p>
          </div>

          <div className="space-y-6">
            {/* Group Settings */}
            {isAdmin && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center">
                    <SettingsIcon className="mr-2 h-5 w-5" />
                    Group Settings
                  </CardTitle>
                  <CardDescription>
                    Manage your family group configuration
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium">Group Name</p>
                      <p className="text-sm text-muted-foreground">
                        {user?.group.name}
                      </p>
                    </div>
                    <Button variant="outline" size="sm">
                      Edit
                    </Button>
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium">Default Member Quota</p>
                      <p className="text-sm text-muted-foreground">
                        Set storage limits for new members
                      </p>
                    </div>
                    <Button variant="outline" size="sm">
                      Configure
                    </Button>
                  </div>

                  <div className="pt-4 border-t">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-medium text-destructive">Delete Group</p>
                        <p className="text-sm text-muted-foreground">
                          Permanently delete this group and all data
                        </p>
                      </div>
                      <Button variant="destructive" size="sm">
                        <Trash2 className="mr-2 h-4 w-4" />
                        Delete
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Vault Storage */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <HardDrive className="mr-2 h-5 w-5" />
                  Vault Storage
                </CardTitle>
                <CardDescription>
                  Storage location and usage information
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Storage Location</p>
                    <p className="text-sm text-muted-foreground font-mono">
                      {health?.drive_path || 'Not available'}
                    </p>
                  </div>
                  <Button variant="outline" size="sm" onClick={handleOpenVaultFolder}>
                    <ExternalLink className="mr-2 h-4 w-4" />
                    Open Folder
                  </Button>
                </div>

                {health?.drive_total_bytes && health?.drive_free_bytes && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <p className="font-medium">Storage Usage</p>
                      <p className="text-sm text-muted-foreground">
                        {formatBytes(health.drive_free_bytes)} free of {formatBytes(health.drive_total_bytes)}
                      </p>
                    </div>
                    <div className="w-full bg-secondary rounded-full h-2">
                      <div 
                        className="bg-primary h-2 rounded-full" 
                        style={{ 
                          width: `${((health.drive_total_bytes - health.drive_free_bytes) / health.drive_total_bytes) * 100}%` 
                        }}
                      />
                    </div>
                  </div>
                )}

                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Encryption</p>
                    <p className="text-sm text-muted-foreground">
                      Files are encrypted with AES-256
                    </p>
                  </div>
                  <div className="text-sm text-green-600 dark:text-green-400">
                    Enabled
                  </div>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Cleanup Actions</p>
                    <p className="text-sm text-muted-foreground">
                      Remove old sessions and temporary files
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Run Cleanup
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Network Settings */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Network className="mr-2 h-5 w-5" />
                  Network
                </CardTitle>
                <CardDescription>
                  Connection and network preferences
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Local Server Port</p>
                    <p className="text-sm text-muted-foreground">
                      Currently running on port 8000
                    </p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    8000
                  </Button>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">LAN Access</p>
                    <p className="text-sm text-muted-foreground">
                      Allow access from other devices on your network
                    </p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    Disabled
                  </Button>
                </div>

                <div>
                  <p className="text-sm text-muted-foreground">
                    💡 For remote access, consider using Tailscale or similar VPN solutions
                  </p>
                </div>
              </CardContent>
            </Card>

            {/* Authentication */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Shield className="mr-2 h-5 w-5" />
                  Authentication
                </CardTitle>
                <CardDescription>
                  Security and authentication settings
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Current Role</p>
                    <p className="text-sm text-muted-foreground capitalize">
                      {user?.claims.role}
                    </p>
                  </div>
                  <div className="text-sm text-muted-foreground">
                    Read-only
                  </div>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Device Token</p>
                    <p className="text-sm text-muted-foreground">
                      Refresh your authentication token
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Refresh Token
                  </Button>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">Sign Out</p>
                    <p className="text-sm text-muted-foreground">
                      Sign out of this device
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Sign Out
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Appearance */}
            <Card>
              <CardHeader>
                <CardTitle>Appearance</CardTitle>
                <CardDescription>
                  Customize the look and feel of the application
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div>
                  <p className="font-medium mb-3">Theme</p>
                  <div className="flex space-x-2">
                    <Button
                      variant={theme === 'light' ? 'default' : 'outline'}
                      size="sm"
                      onClick={() => handleThemeChange('light')}
                    >
                      <Sun className="mr-2 h-4 w-4" />
                      Light
                    </Button>
                    <Button
                      variant={theme === 'dark' ? 'default' : 'outline'}
                      size="sm"
                      onClick={() => handleThemeChange('dark')}
                    >
                      <Moon className="mr-2 h-4 w-4" />
                      Dark
                    </Button>
                    <Button
                      variant={theme === 'system' ? 'default' : 'outline'}
                      size="sm"
                      onClick={() => handleThemeChange('system')}
                    >
                      <Monitor className="mr-2 h-4 w-4" />
                      System
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}