import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input } from '@familyvault/ui';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { CreateGroupRequestSchema, CreateGroupRequest } from '@familyvault/shared';
import { signInViaGroupCreate, whoAmI } from '../lib/auth';
import { useAuthStore } from '../store/auth';

export default function CreateGroup() {
  const navigate = useNavigate();
  const { setUser } = useAuthStore();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const form = useForm<CreateGroupRequest>({
    resolver: zodResolver(CreateGroupRequestSchema),
    defaultValues: {
      name: '',
      owner_display_name: '',
      email: '',
      phone: '',
    },
  });

  const onSubmit = async (data: CreateGroupRequest) => {
    setIsLoading(true);
    setError(null);

    try {
      await signInViaGroupCreate(data, 'Desktop App');
      
      // Get user info after successful creation
      const userInfo = await whoAmI();
      setUser(userInfo);
      
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create group');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        <div className="flex items-center space-x-2">
          <Button variant="ghost" size="sm" asChild>
            <Link to="/welcome">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <h1 className="text-2xl font-bold text-foreground">Create Family Group</h1>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Group Information</CardTitle>
            <CardDescription>
              You'll become the administrator of this group
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <div>
                <label htmlFor="name" className="block text-sm font-medium text-foreground mb-1">
                  Group Name *
                </label>
                <Input
                  id="name"
                  placeholder="My Family"
                  {...form.register('name')}
                />
                {form.formState.errors.name && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.name.message}
                  </p>
                )}
              </div>

              <div>
                <label htmlFor="owner_display_name" className="block text-sm font-medium text-foreground mb-1">
                  Your Name *
                </label>
                <Input
                  id="owner_display_name"
                  placeholder="John Doe"
                  {...form.register('owner_display_name')}
                />
                {form.formState.errors.owner_display_name && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.owner_display_name.message}
                  </p>
                )}
              </div>

              <div>
                <label htmlFor="email" className="block text-sm font-medium text-foreground mb-1">
                  Email (Optional)
                </label>
                <Input
                  id="email"
                  type="email"
                  placeholder="john@example.com"
                  {...form.register('email')}
                />
                {form.formState.errors.email && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.email.message}
                  </p>
                )}
              </div>

              <div>
                <label htmlFor="phone" className="block text-sm font-medium text-foreground mb-1">
                  Phone (Optional)
                </label>
                <Input
                  id="phone"
                  placeholder="+1234567890"
                  {...form.register('phone')}
                />
                {form.formState.errors.phone && (
                  <p className="text-sm text-destructive mt-1">
                    {form.formState.errors.phone.message}
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
                Create Group
              </Button>
            </form>
          </CardContent>
        </Card>

        <div className="text-center text-xs text-muted-foreground">
          <p>As the group administrator, you can:</p>
          <ul className="mt-1 space-y-1">
            <li>• Invite family members</li>
            <li>• Manage sessions and files</li>
            <li>• Control access permissions</li>
          </ul>
        </div>
      </div>
    </div>
  );
}