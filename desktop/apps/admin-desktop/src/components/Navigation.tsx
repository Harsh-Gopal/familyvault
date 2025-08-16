import { Link, useLocation } from 'react-router-dom';
import { 
  Home, 
  FolderOpen, 
  Users, 
  Bell, 
  Settings, 
  User,
  LogOut,
  HardDrive
} from 'lucide-react';
import { Button } from '@familyvault/ui';
import { useAuthStore } from '../store/auth';
import { signOut } from '../lib/auth';
import GroupDropdown from './GroupDropdown';

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: Home },
  { name: 'Vault', href: '/vault', icon: HardDrive },
  { name: 'Sessions', href: '/sessions', icon: FolderOpen },
  { name: 'Members', href: '/members', icon: Users, adminOnly: true },
  { name: 'Notifications', href: '/notifications', icon: Bell, adminOnly: true },
  { name: 'Settings', href: '/settings', icon: Settings },
  { name: 'Profile', href: '/profile', icon: User },
];

export default function Navigation() {
  const location = useLocation();
  const { user } = useAuthStore();
  
  const isAdmin = user?.claims.role === 'admin';

  const handleSignOut = async () => {
    await signOut();
  };

  return (
    <nav className="w-64 bg-card border-r border-border h-screen flex flex-col no-select">
      <div className="p-6 pt-10">
        <GroupDropdown />
      </div>

      <div className="flex-1 px-4">
        <ul className="space-y-2">
          {navigation.map((item) => {
            if (item.adminOnly && !isAdmin) return null;
            
            const isActive = location.pathname === item.href;
            const Icon = item.icon;

            return (
              <li key={item.name}>
                <Link
                  to={item.href}
                  className={`flex items-center px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                  }`}
                >
                  <Icon className="mr-3 h-4 w-4" />
                  {item.name}
                </Link>
              </li>
            );
          })}
        </ul>
      </div>

      <div className="p-4 border-t border-border">
        <div className="flex items-center space-x-3 mb-3">
          <div className="w-8 h-8 bg-primary rounded-full flex items-center justify-center">
            <span className="text-xs font-medium text-primary-foreground">
              {user?.user.display_name.charAt(0).toUpperCase()}
            </span>
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-foreground truncate">
              {user?.user.display_name}
            </p>
            <p className="text-xs text-muted-foreground capitalize">
              {user?.claims.role}
            </p>
          </div>
        </div>
        
        <Button
          variant="ghost"
          size="sm"
          onClick={handleSignOut}
          className="w-full justify-start"
        >
          <LogOut className="mr-2 h-4 w-4" />
          Sign Out
        </Button>
      </div>
    </nav>
  );
}