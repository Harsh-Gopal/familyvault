import { useState } from 'react';
import { ChevronDown, Plus, Users, UserPlus } from 'lucide-react';
import { Button, Input } from '@familyvault/ui';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';

interface GroupDropdownProps {
  className?: string;
}

export default function GroupDropdown({ className = '' }: GroupDropdownProps) {
  const { user } = useAuthStore();
  const [isOpen, setIsOpen] = useState(false);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [showJoinForm, setShowJoinForm] = useState(false);
  const [groupName, setGroupName] = useState('');
  const [joinToken, setJoinToken] = useState('');
  const [loading, setLoading] = useState(false);

  const handleCreateGroup = async () => {
    if (!groupName.trim()) return;
    
    setLoading(true);
    try {
      const api = await getAPI();
      await api.createGroup({ 
        name: groupName.trim(),
        owner_display_name: user?.user.display_name || 'Unknown'
      });
      
      // Navigate to the new group (this would require backend to return the new group info)
      window.location.reload(); // Simple reload for now
    } catch (error) {
      console.error('Failed to create group:', error);
      alert('Failed to create group');
    } finally {
      setLoading(false);
      setShowCreateForm(false);
      setGroupName('');
      setIsOpen(false);
    }
  };

  const handleJoinGroup = async () => {
    if (!joinToken.trim()) return;
    
    setLoading(true);
    try {
      // For now, just show a message since joinGroup API might not be implemented
      alert('Join group functionality will be implemented when the backend API is ready.');
      
      // const api = await getAPI();
      // await api.joinGroup({ token: joinToken.trim() });
      // window.location.reload();
    } catch (error) {
      console.error('Failed to join group:', error);
      alert('Failed to join group. Please check the token.');
    } finally {
      setLoading(false);
      setShowJoinForm(false);
      setJoinToken('');
      setIsOpen(false);
    }
  };

  const handleSwitchGroup = () => {
    // For now, just show a message about switching groups
    alert('Group switching will be available when you have multiple groups.');
    setIsOpen(false);
  };

  return (
    <div className={`relative ${className}`}>
      <div className="flex items-center space-x-2 w-full">
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-bold text-foreground truncate">FamilyVault</h1>
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="flex items-center space-x-1 text-left hover:bg-accent rounded-md px-2 py-1 transition-colors group"
          >
            <p className="text-sm text-muted-foreground truncate">
              {user?.group.name || 'No Group'}
            </p>
            <ChevronDown className={`h-3 w-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
          </button>
        </div>
      </div>

      {isOpen && (
        <>
          {/* Backdrop */}
          <div 
            className="fixed inset-0 z-10" 
            onClick={() => setIsOpen(false)}
          />
          
          {/* Dropdown Menu */}
          <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded-md shadow-lg z-20 min-w-64">
            {!showCreateForm && !showJoinForm ? (
              <div className="p-2 space-y-1">
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => setShowCreateForm(true)}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Create New Group
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => setShowJoinForm(true)}
                >
                  <UserPlus className="mr-2 h-4 w-4" />
                  Join Group
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start"
                  onClick={handleSwitchGroup}
                >
                  <Users className="mr-2 h-4 w-4" />
                  Switch Group
                </Button>
              </div>
            ) : showCreateForm ? (
              <div className="p-3 space-y-3">
                <h3 className="font-medium text-sm">Create New Group</h3>
                <Input
                  placeholder="Group name"
                  value={groupName}
                  onChange={(e) => setGroupName(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleCreateGroup()}
                />
                <div className="flex space-x-2">
                  <Button
                    size="sm"
                    onClick={handleCreateGroup}
                    disabled={!groupName.trim() || loading}
                    className="flex-1"
                  >
                    {loading ? 'Creating...' : 'Create'}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setShowCreateForm(false);
                      setGroupName('');
                    }}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            ) : (
              <div className="p-3 space-y-3">
                <h3 className="font-medium text-sm">Join Group</h3>
                <Input
                  placeholder="Invitation token"
                  value={joinToken}
                  onChange={(e) => setJoinToken(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleJoinGroup()}
                />
                <div className="flex space-x-2">
                  <Button
                    size="sm"
                    onClick={handleJoinGroup}
                    disabled={!joinToken.trim() || loading}
                    className="flex-1"
                  >
                    {loading ? 'Joining...' : 'Join'}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setShowJoinForm(false);
                      setJoinToken('');
                    }}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}