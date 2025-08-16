import axios, { AxiosInstance } from 'axios';
import {
  CreateGroupRequest,
  CreateGroupResponse,
  InviteMemberRequest,
  InviteMemberResponse,
  PairRequest,
  PairResponse,
  ApproveDeviceResponse,
  WhoAmIResponse,
  HealthResponse,
  Group,
  MemberWithUser,
  Session,
  FilesResponse,
  LogsResponse,
  NotifyRequest,
  NotifyResponse,
  UpdateRoleRequest,
  UsageResponse,
  CreateGroupResponseSchema,
  InviteMemberResponseSchema,
  PairResponseSchema,
  ApproveDeviceResponseSchema,
  WhoAmIResponseSchema,
  HealthResponseSchema,
  UsageResponseSchema,
  NotifyResponseSchema,
} from './types.js';

export class FamilyVaultAPI {
  private client: AxiosInstance;
  private getToken: () => Promise<string | null>;

  constructor(baseURL: string, getToken: () => Promise<string | null>) {
    this.getToken = getToken;
    this.client = axios.create({
      baseURL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add auth interceptor
    this.client.interceptors.request.use(async (config) => {
      const token = await this.getToken();
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    // Add response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          // Token expired or invalid - emit event for logout
          window.dispatchEvent(new CustomEvent('auth:logout'));
        }
        return Promise.reject(error);
      }
    );
  }

  // Health check
  async health(): Promise<HealthResponse> {
    const response = await this.client.get('/health');
    return HealthResponseSchema.parse(response.data);
  }

  async version(): Promise<{ version: string }> {
    const response = await this.client.get('/version');
    return response.data;
  }

  // Authentication
  async createGroup(request: CreateGroupRequest, deviceName?: string): Promise<CreateGroupResponse> {
    const headers = deviceName ? { 'X-Device-Name': deviceName } : {};
    const response = await this.client.post('/groups', request, { headers });
    return CreateGroupResponseSchema.parse(response.data);
  }

  async pair(request: PairRequest): Promise<PairResponse> {
    const response = await this.client.post('/pair', request);
    return PairResponseSchema.parse(response.data);
  }

  async whoAmI(): Promise<WhoAmIResponse> {
    const response = await this.client.get('/me');
    return WhoAmIResponseSchema.parse(response.data);
  }

  // Groups
  async listGroups(): Promise<Group[]> {
    const response = await this.client.get('/groups');
    return response.data;
  }

  async getGroup(groupId: string): Promise<{ group: Group; member_count: number; members: MemberWithUser[] }> {
    const response = await this.client.get(`/groups/${groupId}`);
    return response.data;
  }

  // Members
  async inviteMember(groupId: string, request: InviteMemberRequest): Promise<InviteMemberResponse> {
    const response = await this.client.post(`/groups/${groupId}/members/invite`, request);
    return InviteMemberResponseSchema.parse(response.data);
  }

  async listMembers(groupId: string): Promise<MemberWithUser[]> {
    const response = await this.client.get(`/groups/${groupId}/members`);
    return response.data;
  }

  async updateMemberRole(groupId: string, userId: string, request: UpdateRoleRequest): Promise<void> {
    await this.client.post(`/groups/${groupId}/roles/${userId}`, request);
  }

  async removeMember(groupId: string, userId: string): Promise<void> {
    await this.client.delete(`/groups/${groupId}/members/${userId}`);
  }

  async approveDevice(groupId: string, deviceId: string): Promise<ApproveDeviceResponse> {
    const response = await this.client.post(`/groups/${groupId}/devices/${deviceId}/approve`);
    return ApproveDeviceResponseSchema.parse(response.data);
  }

  // Sessions
  async openSession(groupId: string, durationMinutes?: number): Promise<Session> {
    const data = durationMinutes ? { duration_minutes: durationMinutes } : {};
    const response = await this.client.post(`/groups/${groupId}/sessions/open`, data);
    return response.data;
  }

  async closeSession(groupId: string): Promise<void> {
    await this.client.post(`/groups/${groupId}/sessions/close`);
  }

  async getActiveSession(groupId: string): Promise<Session[]> {
    const response = await this.client.get(`/groups/${groupId}/sessions/active`);
    return response.data;
  }

  async getSession(groupId: string, sessionId: string): Promise<Session> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}`);
    return response.data;
  }

  async getSessionStatus(groupId: string, sessionId: string): Promise<{
    session_id: string;
    group_id: string;
    started_by_user: string;
    created_at: string;
    expires: string;
    remaining_seconds: number;
    is_active: boolean;
  }> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}/status`);
    return response.data;
  }

  async deleteSession(groupId: string, sessionId: string): Promise<void> {
    await this.client.delete(`/groups/${groupId}/sessions/${sessionId}`);
  }

  // Files
  async uploadFile(
    groupId: string,
    sessionId: string,
    file: File,
    tags?: Record<string, string>,
    onProgress?: (progress: number) => void
  ): Promise<void> {
    const formData = new FormData();
    formData.append('file', file);
    if (tags) {
      formData.append('tags', JSON.stringify(tags));
    }

    await this.client.post(`/groups/${groupId}/sessions/${sessionId}/files/upload`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = (progressEvent.loaded / progressEvent.total) * 100;
          onProgress(progress);
        }
      },
    });
  }

  async listFiles(
    groupId: string,
    sessionId: string,
    params?: {
      limit?: number;
      offset?: number;
      extension?: string;
      min_size?: number;
      max_size?: number;
      sort_by?: 'name' | 'size' | 'modified_time';
      order?: 'asc' | 'desc';
    }
  ): Promise<FilesResponse> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}/files`, { params });
    return response.data;
  }

  async downloadFile(groupId: string, sessionId: string, filename: string): Promise<Blob> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}/files/${filename}/download`, {
      responseType: 'blob',
    });
    return response.data;
  }

  async deleteFile(groupId: string, sessionId: string, filename: string): Promise<void> {
    await this.client.delete(`/groups/${groupId}/sessions/${sessionId}/files/${filename}`);
  }

  async downloadAll(groupId: string, sessionId: string): Promise<Blob> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}/download-all`, {
      responseType: 'blob',
    });
    return response.data;
  }

  // Logs
  async getLogs(
    groupId: string,
    sessionId: string,
    params?: {
      limit?: number;
      offset?: number;
      tail?: boolean;
      level?: 'debug' | 'info' | 'warn' | 'error';
    }
  ): Promise<LogsResponse> {
    const response = await this.client.get(`/groups/${groupId}/sessions/${sessionId}/logs`, { params });
    return response.data;
  }

  // Notifications
  async notify(groupId: string, request: NotifyRequest): Promise<NotifyResponse> {
    const response = await this.client.post(`/groups/${groupId}/notify`, request);
    return NotifyResponseSchema.parse(response.data);
  }

  // Usage
  async getUsage(groupId: string): Promise<UsageResponse> {
    const response = await this.client.get(`/groups/${groupId}/usage`);
    return UsageResponseSchema.parse(response.data);
  }

  // Search
  async searchFiles(
    params: {
      name?: string;
      type?: string;
      tags?: string;
      date_from?: string;
      date_to?: string;
      session_id?: string;
    }
  ): Promise<{ files: any[] }> {
    const response = await this.client.get('/search-files', { params });
    return response.data;
  }
}