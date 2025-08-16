import { WhoAmIResponse, CreateGroupRequest, CreateGroupResponse, PairRequest, PairResponse } from '@familyvault/shared';
import { getAPI, resetAPI } from './api';

export async function whoAmI(): Promise<WhoAmIResponse | null> {
  try {
    const api = await getAPI();
    return await api.whoAmI();
  } catch (error) {
    console.error('Failed to get user info:', error);
    return null;
  }
}

export async function signInViaGroupCreate(
  request: CreateGroupRequest,
  deviceName?: string
): Promise<CreateGroupResponse> {
  const api = await getAPI();
  const response = await api.createGroup(request, deviceName);
  
  // Store token in keychain
  await window.fv.setToken(response.token);
  
  return response;
}

export async function signInViaPair(request: PairRequest): Promise<PairResponse> {
  const api = await getAPI();
  return await api.pair(request);
}

export async function signOut(): Promise<void> {
  await window.fv.clearToken();
  resetAPI();
  
  // Emit logout event
  window.dispatchEvent(new CustomEvent('auth:logout'));
}

export async function isAuthenticated(): Promise<boolean> {
  const token = await window.fv.getToken();
  return token !== null;
}

export async function waitForApproval(deviceId: string): Promise<string | null> {
  // Poll for approval - in a real app you might use WebSocket or Server-Sent Events
  const maxAttempts = 60; // 5 minutes
  
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const userInfo = await whoAmI();
      if (userInfo && userInfo.claims.device_id === deviceId) {
        return userInfo.claims.role;
      }
    } catch (error) {
      // Continue polling
    }
    
    await new Promise(resolve => setTimeout(resolve, 5000)); // Wait 5 seconds
  }
  
  return null; // Timeout
}