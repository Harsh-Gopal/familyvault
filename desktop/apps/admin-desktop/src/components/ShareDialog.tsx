import { useState, useEffect } from 'react';
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@familyvault/ui';
import { Loader2, CheckCircle } from 'lucide-react';
import { ShareTarget } from '../../electron/preload';

interface ShareDialogProps {
  title: string;
  content: string;
  onClose: () => void;
}

export default function ShareDialog({ title, content, onClose }: ShareDialogProps) {
  const [targets, setTargets] = useState<ShareTarget[]>([]);
  const [loading, setLoading] = useState(true);
  const [sharing, setSharing] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    loadTargets();
  }, []);

  const loadTargets = async () => {
    try {
      const result = await window.fv.share.listTargets();
      if (result.ok && result.data) {
        setTargets(result.data);
      }
    } catch (error) {
      console.error('Failed to load share targets:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleShare = async (targetId: string) => {
    if (targetId === 'copy') {
      try {
        await window.fv.copyToClipboard(content);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      } catch (error) {
        console.error('Failed to copy to clipboard:', error);
      }
      return;
    }

    setSharing(targetId);
    try {
      const result = await window.fv.share.invoke(targetId, content);
      if (!result.ok) {
        alert(`Failed to share: ${result.error}`);
      }
    } catch (error) {
      console.error('Failed to share:', error);
      alert('Failed to share');
    } finally {
      setSharing(null);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 backdrop-blur-sm">
      <Card className="w-full max-w-md mx-4 bg-white/95 dark:bg-gray-900/95 backdrop-blur-xl rounded-2xl shadow-2xl border-0">
        <CardHeader className="text-center pb-4">
          <CardTitle className="text-xl font-semibold">{title}</CardTitle>
          <CardDescription className="text-sm">
            Choose how to share this invitation
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Content Preview */}
          <div className="bg-gray-50 dark:bg-gray-800 rounded-xl p-4 text-sm font-mono text-gray-700 dark:text-gray-300 max-h-32 overflow-y-auto">
            {content}
          </div>

          {/* Share Targets */}
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin" />
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-3">
              {targets.map((target) => (
                <Button
                  key={target.id}
                  variant="outline"
                  className="h-auto p-4 flex flex-col items-center space-y-2 hover:bg-gray-50 dark:hover:bg-gray-800 transition-all duration-200 rounded-xl border-gray-200 dark:border-gray-700"
                  onClick={() => handleShare(target.id)}
                  disabled={sharing === target.id}
                >
                  {sharing === target.id ? (
                    <Loader2 className="h-6 w-6 animate-spin" />
                  ) : target.id === 'copy' && copied ? (
                    <CheckCircle className="h-6 w-6 text-green-600" />
                  ) : (
                    <span className="text-2xl">{target.icon}</span>
                  )}
                  <span className="text-sm font-medium">
                    {target.id === 'copy' && copied ? 'Copied!' : target.name}
                  </span>
                </Button>
              ))}
            </div>
          )}

          {/* Close Button */}
          <div className="flex justify-center pt-4">
            <Button
              variant="ghost"
              onClick={onClose}
              className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            >
              Close
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}