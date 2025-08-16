import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input } from '@familyvault/ui';
import { ArrowLeft, Loader2, Clock } from 'lucide-react';
import { PairRequestSchema, PairRequest } from '@familyvault/shared';
import { signInViaPair, waitForApproval, whoAmI } from '../lib/auth';
import { useAuthStore } from '../store/auth';

export default function Pair() {
  const navigate = useNavigate();
  const { setUser } = useAuthStore();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pairResponse, setPairResponse] = useState<any>(null);
  const [isWaitingForApproval, setIsWaitingForApproval] = useState(false);

  const form = useForm<PairRequest>({
    resolver: zodResolver(PairRequestSchema),
    defaultValues: {
      token: '',
      device_name: '',
    },
  });

  // Handle deep link from QR code
  useEffect(() => {
    const handleDeepLink = (url: string) => {
      try {
        const urlObj = new URL(url);
        if (urlObj.protocol === 'familyvault:' && urlObj.pathname === '//pair') {
          const token = urlObj.searchParams.get('token');
          if (token) {
            form.setValue('token', token);
          }
        }
      } catch (error) {
        console.error('Failed to parse deep link:', error);
      }
    };

    window.fv.onDeepLink(handleDeepLink);

    return () => {
      window.fv.removeAllListeners('deep-link');
    };
  }, [form]);

  const onSubmit = async (data: PairRequest) => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await signInViaPair(data);
      setPairResponse(response);
      
      if (response.pending) {
        setIsWaitingForApproval(true);
        // Wait for admin approval
        const role = await waitForApproval(response.device_id);
        
        if (role) {
          // Get user info after approval
          const userInfo = await whoAmI();
          setUser(userInfo);
          navigate('/dashboard');
        } else {
          setError('Approval timeout. Please try again or contact your administrator.');
          setIsWaitingForApproval(false);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to pair device');
    } finally {
      setIsLoading(false);
    }
  };

  if (isWaitingForApproval) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <Clock className="h-12 w-12 text-yellow-500 mx-auto mb-4" />
            <CardTitle>Waiting for Approval</CardTitle>
            <CardDescription>
              Your device pairing request has been sent to the group administrator
            </CardDescription>
          </CardHeader>
          <CardContent className="text-center space-y-4">
            <div className="flex items-center justify-center space-x-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-sm text-muted-foreground">
                Waiting for admin approval...
              </span>
            </div>
            
            <div className="p-4 bg-muted rounded-lg">
              <p className="text-sm text-muted-foreground">
                <strong>Group:</strong> {pairResponse?.group_id}
              </p>
              <p className="text-sm text-muted-foreground">
                <strong>Device:</strong> {pairResponse?.device_id}
              </p>
            </div>

            <Button
              variant="outline"
              onClick={() => {
                setIsWaitingForApproval(false);
                setPairResponse(null);
                setError(null);
              }}
            >
              Cancel
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        <div className="flex items-center space-x-2">
          <Button variant="ghost" size="sm" asChild>
            <Link to="/welcome">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <h1 className="text-2xl font-bold text-foreground">Join Family Group</h1>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Device Pairing</CardTitle>
            <CardDescription>
              Enter your invitation token to join the family group
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <div>
                <label htmlFor="token" className="block text-sm font-medium text-foreground mb-1">
                  Pairing Token *
                </label>
                <Input
                  id="token"
                  placeholder="Paste your invitation token here"
                  {...form.register('token')}
                />
                {form.formState.errors.token && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.token.message}
                  </p>
                )}
                <p className="text-xs text-muted-foreground mt-1">
                  You can also scan a QR code to auto-fill this field
                </p>
              </div>

              <div>
                <label htmlFor="device_name" className="block text-sm font-medium text-foreground mb-1">
                  Device Name *
                </label>
                <Input
                  id="device_name"
                  placeholder="My MacBook"
                  {...form.register('device_name')}
                />
                {form.formState.errors.device_name && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.device_name.message}
                  </p>
                )}
              </div>

              {error && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
                  <p className="text-sm text-destructive">{error}</p>
                </div>
              )}

              <Button type="submit" className="w-full" disabled={isLoading}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Pair Device
              </Button>
            </form>
          </CardContent>
        </Card>

        <div className="text-center text-xs text-muted-foreground">
          <p>After pairing, an administrator must approve your device</p>
          <p>You'll be notified when approval is complete</p>
        </div>
      </div>
    </div>
  );
}