import Navigation from '../components/Navigation';
import VaultManager from '../components/VaultManager';

export default function Vault() {
  return (
    <div className="flex h-screen bg-background">
      <Navigation />
      
      <main className="flex-1 overflow-auto pt-8">
        <div className="p-6">
          <div className="mb-6">
            <h1 className="text-2xl font-bold text-foreground">Vault</h1>
            <p className="text-muted-foreground">
              Manage your storage location and upload files
            </p>
          </div>

          <VaultManager />
        </div>
      </main>
    </div>
  );
}