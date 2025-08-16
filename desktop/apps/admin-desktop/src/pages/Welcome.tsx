import { Link } from 'react-router-dom';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { Users, Plus } from 'lucide-react';

export default function Welcome() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-foreground">FamilyVault</h1>
          <p className="text-muted-foreground mt-2">
            Secure family file sharing and storage
          </p>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Plus className="mr-2 h-5 w-5" />
                Create Family Group
              </CardTitle>
              <CardDescription>
                Start a new family vault and become the administrator
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button asChild className="w-full">
                <Link to="/create-group">Create Group</Link>
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Users className="mr-2 h-5 w-5" />
                Join Family Group
              </CardTitle>
              <CardDescription>
                Join an existing family vault with an invitation
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button asChild variant="outline" className="w-full">
                <Link to="/pair">Join Group</Link>
              </Button>
            </CardContent>
          </Card>
        </div>

        <div className="text-center text-xs text-muted-foreground">
          <p>All data is stored locally on your device</p>
          <p>No cloud services required</p>
        </div>
      </div>
    </div>
  );
}