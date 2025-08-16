import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useSearchParams } from 'react-router-dom';
import { 
  Button, 
  Card, 
  CardContent, 
  CardDescription, 
  CardHeader, 
  CardTitle,
  Input
} from '@familyvault/ui';
import { 
  UserPlus, 
  Users, 
  Loader2,
  MoreHorizontal,
  Shield,
  UserCheck
} from 'lucide-react';
import Navigation from '../components/Navigation';
import ShareDialog from '../components/ShareDialog';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';
import { 
  InviteMemberRequestSchema, 
  InviteMemberRequest,
  UpdateRoleRequest 
} from '@familyvault/shared';
import { formatDateTime, getRoleColor, getStatusColor } from '../lib/utils';

export default function Members() {
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();
  const [showInviteForm, setShowInviteForm] = useState(false);
  const [shareDialog, setShareDialog] = useState<{ title: string; content: string } | null>(null);

  const isAdmin = user?.claims.role === 'admin';

  // Auto-show invite form if navigated from dashboard
  useEffect(() => {
    if (searchParams.get('invite') === 'true') {
      setShowInviteForm(true);
    }
  }, [searchParams]);

  // Members query
  const { data: members, isLoading } = useQuery({
    queryKey: ['members', user?.claims.group_id],
    queryFn: async () => {
      if (!user?.claims.group_id) return [];
      const api = await getAPI();
      return api.listMembers(user.claims.group_id);
    },
    enabled: !!user?.claims.group_id,
  });

  // Invite form
  const inviteForm = useForm<InviteMemberRequest>({
    resolver: zodResolver(InviteMemberRequestSchema),
    defaultValues: {
      contact: '',
      ttl_minutes: 60,
    },
  });

  // Invite mutation
  const inviteMutation = useMutation({
    mutationFn: async (data: InviteMemberRequest) => {
      if (!user?.claims.group_id) throw new Error('No group ID');
      const api = await getAPI();
      return api.inviteMember(user.claims.group_id, data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
      inviteForm.reset();
    },
  });

  // Role update mutation
  const updateRoleMutation = useMutation({
    mutationFn: async ({ userId, role }: { userId: string; role: string }) => {
      if (!user?.claims.group_id) throw new Error('No group ID');
      const api = await getAPI();
      return api.updateMemberRole(user.claims.group_id, userId, { role } as UpdateRoleRequest);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
    },
  });

  const handleInvite = async (data: InviteMemberRequest) => {
    const result = await inviteMutation.mutateAsync(data);
    
    // Create share content
    const shareText = `Join our FamilyVault family group!\n\nPairing token: ${result.pairing_token}\n\nQR Code: ${result.qr}`;
    
    // Show share dialog
    setShareDialog({
      title: 'Share Invitation',
      content: shareText
    });
  };



  const handleRoleChange = async (userId: string, newRole: string) => {
    await updateRoleMutation.mutateAsync({ userId, role: newRole });
  };

  if (!isAdmin) {
    return (
      <div className="flex h-screen bg-background">
        <Navigation />
        <main className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <Shield className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
            <p className="text-muted-foreground">Admin access required</p>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto pt-8">
        <div className="p-6">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold text-foreground">Members</h1>
              <p className="text-muted-foreground">
                Manage family group members and permissions
              </p>
            </div>
            
            <Button onClick={() => setShowInviteForm(true)}>
              <UserPlus className="mr-2 h-4 w-4" />
              Add Member
            </Button>
          </div>

          {/* Invite Form */}
          {showInviteForm && (
            <Card className="mb-6">
              <CardHeader>
                <CardTitle>Add New Member</CardTitle>
                <CardDescription>
                  Send an invitation to join your family group
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={inviteForm.handleSubmit(handleInvite)} className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label htmlFor="contact" className="block text-sm font-medium text-foreground mb-1">
                        Email or Phone *
                      </label>
                      <Input
                        id="contact"
                        placeholder="john@example.com or +1234567890"
                        {...inviteForm.register('contact')}
                      />
                      {inviteForm.formState.errors.contact && (
                        <p className="text-sm text-destructive mt-1">
                          {inviteForm.formState.errors.contact.message}
                        </p>
                      )}
                    </div>

                    <div>
                      <label htmlFor="ttl_minutes" className="block text-sm font-medium text-foreground mb-1">
                        Token Expires (minutes)
                      </label>
                      <Input
                        id="ttl_minutes"
                        type="number"
                        min="1"
                        max="1440"
                        {...inviteForm.register('ttl_minutes', { valueAsNumber: true })}
                      />
                    </div>
                  </div>

                  <div className="flex space-x-2">
                    <Button 
                      type="submit" 
                      disabled={inviteMutation.isPending}
                    >
                      {inviteMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                      Send Invitation
                    </Button>
                    <Button 
                      type="button" 
                      variant="outline"
                      onClick={() => setShowInviteForm(false)}
                    >
                      Cancel
                    </Button>
                  </div>
                </form>
              </CardContent>
            </Card>
          )}

          {/* Members List */}
          <Card>
            <CardHeader>
              <CardTitle>Group Members</CardTitle>
              <CardDescription>
                {members?.length || 0} members in your family group
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin" />
                </div>
              ) : !members || members.length === 0 ? (
                <div className="text-center py-8">
                  <Users className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">No members found</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {members.map((member) => (
                    <div
                      key={member.user.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex items-center space-x-4">
                        <div className="w-10 h-10 bg-primary rounded-full flex items-center justify-center">
                          <span className="text-sm font-medium text-primary-foreground">
                            {member.user.display_name.charAt(0).toUpperCase()}
                          </span>
                        </div>
                        
                        <div>
                          <div className="flex items-center space-x-2">
                            <h3 className="font-medium">{member.user.display_name}</h3>
                            <span className={`px-2 py-1 text-xs rounded-full ${getRoleColor(member.user.id === user?.claims.user_id ? 'admin' : member.membership.role)}`}>
                              {member.user.id === user?.claims.user_id ? 'admin' : member.membership.role}
                            </span>
                            <span className={`px-2 py-1 text-xs rounded-full ${getStatusColor(member.membership.status)}`}>
                              {member.membership.status}
                            </span>
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {member.user.email && <span>{member.user.email}</span>}
                            {member.user.phone && <span> • {member.user.phone}</span>}
                          </div>
                          <p className="text-xs text-muted-foreground">
                            Joined {formatDateTime(member.membership.created_at)}
                          </p>
                        </div>
                      </div>

                      <div className="flex items-center space-x-2">
                        {member.membership.status === 'active' && (
                          <>
                            {/* Prevent group creator (current user) from changing their own role */}
                            {member.user.id === user?.claims.user_id ? (
                              <span className="text-sm px-2 py-1 bg-muted rounded">
                                admin (You)
                              </span>
                            ) : (
                              <select
                                value={member.membership.role}
                                onChange={(e) => handleRoleChange(member.user.id, e.target.value)}
                                className="text-sm border rounded px-2 py-1"
                                disabled={updateRoleMutation.isPending}
                              >
                                <option value="viewer">Viewer</option>
                                <option value="member">Member</option>
                                <option value="admin">Admin</option>
                              </select>
                            )}
                          </>
                        )}

                        {member.membership.status === 'pending' && (
                          <Button size="sm" variant="outline">
                            <UserCheck className="h-4 w-4" />
                          </Button>
                        )}

                        <Button variant="outline" size="sm">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </main>

      {/* Share Dialog */}
      {shareDialog && (
        <ShareDialog
          title={shareDialog.title}
          content={shareDialog.content}
          onClose={() => setShareDialog(null)}
        />
      )}
    </div>
  );
}