import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation } from '@tanstack/react-query';
import { 
  Button, 
  Card, 
  CardContent, 
  CardDescription, 
  CardHeader, 
  CardTitle
} from '@familyvault/ui';
import { Bell, Send, Mail, MessageSquare, Loader2, Shield } from 'lucide-react';
import Navigation from '../components/Navigation';
import { useAuthStore } from '../store/auth';
import { getAPI } from '../lib/api';
import { NotifyRequestSchema, NotifyRequest } from '@familyvault/shared';

export default function Notifications() {
  const { user } = useAuthStore();
  const [lastResult, setLastResult] = useState<{ sent: number; failed: number } | null>(null);

  const isAdmin = user?.claims.role === 'admin';

  const form = useForm<NotifyRequest>({
    resolver: zodResolver(NotifyRequestSchema),
    defaultValues: {
      message: '',
      channels: ['email'],
    },
  });

  const notifyMutation = useMutation({
    mutationFn: async (data: NotifyRequest) => {
      if (!user?.claims.group_id) throw new Error('No group ID');
      const api = await getAPI();
      return api.notify(user.claims.group_id, data);
    },
    onSuccess: (result) => {
      setLastResult(result);
      form.reset();
    },
  });

  const handleSubmit = async (data: NotifyRequest) => {
    await notifyMutation.mutateAsync(data);
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
      
      <main className="flex-1 overflow-auto">
        <div className="p-6">
          <div className="mb-6">
            <h1 className="text-2xl font-bold text-foreground">Notifications</h1>
            <p className="text-muted-foreground">
              Send messages to all family group members
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Send Notification */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center">
                  <Send className="mr-2 h-5 w-5" />
                  Send Notification
                </CardTitle>
                <CardDescription>
                  Notify all active members in your family group
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
                  <div>
                    <label htmlFor="message" className="block text-sm font-medium text-foreground mb-1">
                      Message *
                    </label>
                    <textarea
                      id="message"
                      rows={4}
                      placeholder="The family vault is now online and ready for uploads..."
                      className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                      {...form.register('message')}
                    />
                    {form.formState.errors.message && (
                      <p className="text-sm text-destructive mt-1">
                        {form.formState.errors.message.message}
                      </p>
                    )}
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-2">
                      Delivery Channels
                    </label>
                    <div className="space-y-2">
                      <label className="flex items-center space-x-2">
                        <input
                          type="checkbox"
                          value="email"
                          {...form.register('channels')}
                          className="rounded border-input"
                        />
                        <Mail className="h-4 w-4" />
                        <span className="text-sm">Email</span>
                      </label>
                      <label className="flex items-center space-x-2">
                        <input
                          type="checkbox"
                          value="sms"
                          {...form.register('channels')}
                          className="rounded border-input"
                        />
                        <MessageSquare className="h-4 w-4" />
                        <span className="text-sm">SMS</span>
                      </label>
                    </div>
                    {form.formState.errors.channels && (
                      <p className="text-sm text-destructive mt-1">
                        {form.formState.errors.channels.message}
                      </p>
                    )}
                  </div>

                  <Button 
                    type="submit" 
                    className="w-full"
                    disabled={notifyMutation.isPending}
                  >
                    {notifyMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    <Send className="mr-2 h-4 w-4" />
                    Send Notification
                  </Button>
                </form>

                {notifyMutation.error && (
                  <div className="mt-4 p-3 bg-destructive/10 border border-destructive/20 rounded-md">
                    <p className="text-sm text-destructive">
                      {notifyMutation.error instanceof Error 
                        ? notifyMutation.error.message 
                        : 'Failed to send notification'
                      }
                    </p>
                  </div>
                )}

                {lastResult && (
                  <div className="mt-4 p-3 bg-green-50 border border-green-200 rounded-md dark:bg-green-950 dark:border-green-800">
                    <p className="text-sm text-green-800 dark:text-green-200">
                      Notification sent successfully!
                    </p>
                    <p className="text-xs text-green-600 dark:text-green-400 mt-1">
                      {lastResult.sent} delivered, {lastResult.failed} failed
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Quick Messages */}
            <Card>
              <CardHeader>
                <CardTitle>Quick Messages</CardTitle>
                <CardDescription>
                  Common notification templates
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <Button
                  variant="outline"
                  className="w-full justify-start text-left h-auto p-3"
                  onClick={() => form.setValue('message', 'The family vault is now online and ready for file uploads!')}
                >
                  <div>
                    <p className="font-medium">Vault Online</p>
                    <p className="text-xs text-muted-foreground">
                      Notify when the vault comes online
                    </p>
                  </div>
                </Button>

                <Button
                  variant="outline"
                  className="w-full justify-start text-left h-auto p-3"
                  onClick={() => form.setValue('message', 'New upload session has started. You can now upload your files.')}
                >
                  <div>
                    <p className="font-medium">Session Started</p>
                    <p className="text-xs text-muted-foreground">
                      Notify when a new session begins
                    </p>
                  </div>
                </Button>

                <Button
                  variant="outline"
                  className="w-full justify-start text-left h-auto p-3"
                  onClick={() => form.setValue('message', 'Upload session will close in 30 minutes. Please finish your uploads.')}
                >
                  <div>
                    <p className="font-medium">Session Ending</p>
                    <p className="text-xs text-muted-foreground">
                      Warn about session expiration
                    </p>
                  </div>
                </Button>

                <Button
                  variant="outline"
                  className="w-full justify-start text-left h-auto p-3"
                  onClick={() => form.setValue('message', 'Your files are ready for download. The vault will be available for the next 24 hours.')}
                >
                  <div>
                    <p className="font-medium">Files Ready</p>
                    <p className="text-xs text-muted-foreground">
                      Notify when files are ready
                    </p>
                  </div>
                </Button>
              </CardContent>
            </Card>
          </div>

          {/* Recent Notifications */}
          <Card className="mt-6">
            <CardHeader>
              <CardTitle>Recent Notifications</CardTitle>
              <CardDescription>
                History of sent notifications
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-center py-8">
                <Bell className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <p className="text-muted-foreground">No recent notifications</p>
              </div>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}