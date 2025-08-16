import { useState } from 'react';
import { Button } from '@familyvault/ui';
import { Play, Square, Loader2 } from 'lucide-react';
import { useAuthStore } from '../store/auth';
import { useSessionStore } from '../store/session';
import { getAPI } from '../lib/api';

interface SessionControlProps {
  onSessionChange?: () => void;
}

export default function SessionControl({ onSessionChange }: SessionControlProps) {
  const { user } = useAuthStore();
  const { activeSession, setActiveSession } = useSessionStore();
  const [isStarting, setIsStarting] = useState(false);
  const [isStopping, setIsStopping] = useState(false);

  const isAdmin = user?.claims.role === 'admin';

  if (!isAdmin) return null;

  const handleStartSession = async () => {
    if (!user?.claims.group_id) return;
    
    setIsStarting(true);
    try {
      const api = await getAPI();
      const session = await api.openSession(user.claims.group_id);
      setActiveSession(session);
      onSessionChange?.();
    } catch (error) {
      console.error('Failed to start session:', error);
    } finally {
      setIsStarting(false);
    }
  };

  const handleStopSession = async () => {
    if (!user?.claims.group_id) return;
    
    setIsStopping(true);
    try {
      const api = await getAPI();
      await api.closeSession(user.claims.group_id);
      setActiveSession(null);
      onSessionChange?.();
    } catch (error) {
      console.error('Failed to stop session:', error);
    } finally {
      setIsStopping(false);
    }
  };

  return (
    <div className="flex items-center space-x-3">
      <div className="flex items-center space-x-2">
        <div className={`w-2 h-2 rounded-full ${
          activeSession ? 'bg-green-500 animate-pulse' : 'bg-gray-400'
        }`} />
        <span className="text-sm font-medium text-muted-foreground">
          {activeSession ? 'Active' : 'Stopped'}
        </span>
      </div>
      
      {!activeSession ? (
        <Button
          size="sm"
          onClick={handleStartSession}
          disabled={isStarting}
          className="h-8 px-4 bg-green-600 hover:bg-green-700 text-white rounded-full transition-all duration-300 transform hover:scale-105"
        >
          {isStarting ? (
            <Loader2 className="mr-1 h-3 w-3 animate-spin" />
          ) : (
            <Play className="mr-1 h-3 w-3" />
          )}
          <span className="text-xs font-medium">
            {isStarting ? 'Starting...' : 'Start'}
          </span>
        </Button>
      ) : (
        <Button
          size="sm"
          onClick={handleStopSession}
          disabled={isStopping}
          className="h-8 px-4 bg-red-600 hover:bg-red-700 text-white rounded-full transition-all duration-300 transform hover:scale-105"
        >
          {isStopping ? (
            <Loader2 className="mr-1 h-3 w-3 animate-spin" />
          ) : (
            <Square className="mr-1 h-3 w-3" />
          )}
          <span className="text-xs font-medium">
            {isStopping ? 'Stopping...' : 'Stop'}
          </span>
        </Button>
      )}
    </div>
  );
}