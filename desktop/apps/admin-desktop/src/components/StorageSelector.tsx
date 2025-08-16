import { useState } from 'react';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input } from '@familyvault/ui';
import { 
  HardDrive, 
  Folder, 
  Lock, 
  Unlock,
  ExternalLink,
  Smartphone,
  Monitor
} from 'lucide-react';

interface StorageSelectorProps {
  onStorageSelect: (storage: StorageConfig) => void;
  className?: string;
}

interface StorageConfig {
  type: 'internal' | 'external';
  path: string;
  encrypted: boolean;
  password?: string;
  device: 'desktop' | 'mobile';
}

export default function StorageSelector({ onStorageSelect, className = '' }: StorageSelectorProps) {
  const [selectedStorage, setSelectedStorage] = useState<StorageConfig>({
    type: 'internal',
    path: '',
    encrypted: false,
    device: 'desktop'
  });
  const [showPasswordInput, setShowPasswordInput] = useState(false);

  const handleStorageTypeChange = (type: 'internal' | 'external') => {
    setSelectedStorage(prev => ({
      ...prev,
      type,
      path: type === 'internal' ? '/Users/Documents/FamilyVault' : ''
    }));
  };

  const handleEncryptionToggle = () => {
    const encrypted = !selectedStorage.encrypted;
    setSelectedStorage(prev => ({ ...prev, encrypted }));
    setShowPasswordInput(encrypted);
  };

  const handlePasswordChange = (password: string) => {
    setSelectedStorage(prev => ({ ...prev, password }));
  };

  const handlePathSelect = async () => {
    // In a real implementation, this would open a file dialog
    // For now, we'll simulate it
    const path = prompt('Enter storage path:', selectedStorage.path);
    if (path) {
      setSelectedStorage(prev => ({ ...prev, path }));
    }
  };

  const handleConfirm = () => {
    if (!selectedStorage.path) {
      alert('Please select a storage location');
      return;
    }
    if (selectedStorage.encrypted && !selectedStorage.password) {
      alert('Please enter an encryption password');
      return;
    }
    onStorageSelect(selectedStorage);
  };

  return (
    <Card className={`w-full max-w-md ${className}`}>
      <CardHeader>
        <CardTitle className="flex items-center">
          <HardDrive className="mr-2 h-5 w-5" />
          Storage Configuration
        </CardTitle>
        <CardDescription>
          Choose where to store your backup files
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Device Type */}
        <div>
          <label className="text-sm font-medium mb-2 block">Device Type</label>
          <div className="grid grid-cols-2 gap-2">
            <Button
              variant={selectedStorage.device === 'desktop' ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedStorage(prev => ({ ...prev, device: 'desktop' }))}
              className="flex items-center justify-center"
            >
              <Monitor className="mr-2 h-4 w-4" />
              Desktop
            </Button>
            <Button
              variant={selectedStorage.device === 'mobile' ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedStorage(prev => ({ ...prev, device: 'mobile' }))}
              className="flex items-center justify-center"
            >
              <Smartphone className="mr-2 h-4 w-4" />
              Mobile
            </Button>
          </div>
        </div>

        {/* Storage Type */}
        <div>
          <label className="text-sm font-medium mb-2 block">Storage Location</label>
          <div className="grid grid-cols-2 gap-2">
            <Button
              variant={selectedStorage.type === 'internal' ? 'default' : 'outline'}
              size="sm"
              onClick={() => handleStorageTypeChange('internal')}
              className="flex items-center justify-center"
            >
              <HardDrive className="mr-2 h-4 w-4" />
              Internal
            </Button>
            <Button
              variant={selectedStorage.type === 'external' ? 'default' : 'outline'}
              size="sm"
              onClick={() => handleStorageTypeChange('external')}
              className="flex items-center justify-center"
            >
              <ExternalLink className="mr-2 h-4 w-4" />
              External
            </Button>
          </div>
        </div>

        {/* Path Selection */}
        <div>
          <label className="text-sm font-medium mb-2 block">Storage Path</label>
          <div className="flex space-x-2">
            <Input
              value={selectedStorage.path}
              onChange={(e) => setSelectedStorage(prev => ({ ...prev, path: e.target.value }))}
              placeholder="Select storage location..."
              className="flex-1"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handlePathSelect}
              className="flex items-center"
            >
              <Folder className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Encryption */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-sm font-medium">Encryption</label>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleEncryptionToggle}
              className="flex items-center"
            >
              {selectedStorage.encrypted ? (
                <Lock className="mr-2 h-4 w-4 text-green-600" />
              ) : (
                <Unlock className="mr-2 h-4 w-4 text-gray-400" />
              )}
              {selectedStorage.encrypted ? 'Enabled' : 'Disabled'}
            </Button>
          </div>
          
          {showPasswordInput && (
            <Input
              type="password"
              placeholder="Enter encryption password..."
              value={selectedStorage.password || ''}
              onChange={(e) => handlePasswordChange(e.target.value)}
              className="mt-2"
            />
          )}
        </div>

        {/* Confirm Button */}
        <Button
          onClick={handleConfirm}
          className="w-full"
          disabled={!selectedStorage.path}
        >
          Configure Storage
        </Button>
      </CardContent>
    </Card>
  );
}