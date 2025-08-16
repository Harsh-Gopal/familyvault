import { z } from 'zod';

// Role types
export const RoleSchema = z.enum(['admin', 'member', 'viewer']);
export type Role = z.infer<typeof RoleSchema>;

export const MembershipStatusSchema = z.enum(['active', 'pending', 'revoked']);
export type MembershipStatus = z.infer<typeof MembershipStatusSchema>;

// User types
export const UserSchema = z.object({
  id: z.string(),
  display_name: z.string(),
  email: z.string().optional(),
  phone: z.string().optional(),
  created_at: z.string(),
});
export type User = z.infer<typeof UserSchema>;

// Group types
export const GroupSchema = z.object({
  id: z.string(),
  name: z.string(),
  owner_user: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});
export type Group = z.infer<typeof GroupSchema>;

// Membership types
export const MembershipSchema = z.object({
  group_id: z.string(),
  user_id: z.string(),
  role: RoleSchema,
  status: MembershipStatusSchema,
  quota_bytes: z.number().optional(),
  used_bytes: z.number().optional(),
  created_at: z.string(),
  updated_at: z.string(),
});
export type Membership = z.infer<typeof MembershipSchema>;

// Device types
export const DeviceSchema = z.object({
  id: z.string(),
  user_id: z.string(),
  group_id: z.string(),
  name: z.string(),
  approved: z.boolean(),
  last_seen: z.string(),
});
export type Device = z.infer<typeof DeviceSchema>;

// Session types
export const SessionSchema = z.object({
  session_id: z.string(),
  group_id: z.string(),
  started_by_user: z.string(),
  created_at: z.string(),
  expires: z.string(),
});
export type Session = z.infer<typeof SessionSchema>;

// JWT Claims
export const JWTClaimsSchema = z.object({
  group_id: z.string(),
  user_id: z.string(),
  device_id: z.string(),
  role: RoleSchema,
  iss: z.string(),
  sub: z.string(),
  exp: z.number(),
  nbf: z.number(),
  iat: z.number(),
  jti: z.string(),
});
export type JWTClaims = z.infer<typeof JWTClaimsSchema>;

// API Response types
export const CreateGroupResponseSchema = z.object({
  group_id: z.string(),
  user_id: z.string(),
  device_id: z.string(),
  role: RoleSchema,
  token: z.string(),
});
export type CreateGroupResponse = z.infer<typeof CreateGroupResponseSchema>;

export const InviteMemberResponseSchema = z.object({
  pairing_token: z.string(),
  qr: z.string(),
});
export type InviteMemberResponse = z.infer<typeof InviteMemberResponseSchema>;

export const PairResponseSchema = z.object({
  pending: z.boolean(),
  group_id: z.string(),
  user_id: z.string(),
  device_id: z.string(),
});
export type PairResponse = z.infer<typeof PairResponseSchema>;

export const ApproveDeviceResponseSchema = z.object({
  token: z.string(),
  role: RoleSchema,
});
export type ApproveDeviceResponse = z.infer<typeof ApproveDeviceResponseSchema>;

export const WhoAmIResponseSchema = z.object({
  claims: JWTClaimsSchema,
  user: UserSchema,
  group: GroupSchema,
  membership: MembershipSchema,
});
export type WhoAmIResponse = z.infer<typeof WhoAmIResponseSchema>;

export const HealthResponseSchema = z.object({
  status: z.string(),
  drive_path: z.string().optional(),
  drive_free_bytes: z.number().optional(),
  drive_total_bytes: z.number().optional(),
  data_path: z.string().optional(),
  version: z.string().optional(),
});
export type HealthResponse = z.infer<typeof HealthResponseSchema>;

// File types
export const FileEntrySchema = z.object({
  filename: z.string(),
  size: z.number(),
  modified_time: z.string(),
  extension: z.string(),
  tags: z.record(z.string()).optional(),
});
export type FileEntry = z.infer<typeof FileEntrySchema>;

export const FilesResponseSchema = z.object({
  session_id: z.string(),
  files: z.array(FileEntrySchema),
  total_count: z.number(),
  page_size: z.number(),
  offset: z.number(),
});
export type FilesResponse = z.infer<typeof FilesResponseSchema>;

// Log types
export const LogEntrySchema = z.object({
  timestamp: z.string(),
  level: z.string(),
  message: z.string(),
});
export type LogEntry = z.infer<typeof LogEntrySchema>;

export const LogsResponseSchema = z.object({
  session_id: z.string(),
  log_count: z.number(),
  logs: z.array(LogEntrySchema),
});
export type LogsResponse = z.infer<typeof LogsResponseSchema>;

// Notification types
export const NotifyResponseSchema = z.object({
  sent: z.number(),
  failed: z.number(),
});
export type NotifyResponse = z.infer<typeof NotifyResponseSchema>;

// Request types
export const CreateGroupRequestSchema = z.object({
  name: z.string().min(1),
  owner_display_name: z.string().min(1),
  email: z.string().email().optional(),
  phone: z.string().optional(),
});
export type CreateGroupRequest = z.infer<typeof CreateGroupRequestSchema>;

export const InviteMemberRequestSchema = z.object({
  contact: z.string().min(1),
  ttl_minutes: z.number().min(1).max(1440).optional(),
});
export type InviteMemberRequest = z.infer<typeof InviteMemberRequestSchema>;

export const PairRequestSchema = z.object({
  token: z.string().min(1),
  device_name: z.string().min(1),
});
export type PairRequest = z.infer<typeof PairRequestSchema>;

export const UpdateRoleRequestSchema = z.object({
  role: RoleSchema,
});
export type UpdateRoleRequest = z.infer<typeof UpdateRoleRequestSchema>;

export const NotifyRequestSchema = z.object({
  message: z.string().min(1),
  channels: z.array(z.enum(['email', 'sms'])),
});
export type NotifyRequest = z.infer<typeof NotifyRequestSchema>;

// Error types
export const APIErrorSchema = z.object({
  error: z.string(),
  message: z.string().optional(),
  code: z.number().optional(),
});
export type APIError = z.infer<typeof APIErrorSchema>;

// Usage types
export const UsageEntrySchema = z.object({
  user_id: z.string(),
  display_name: z.string(),
  role: RoleSchema,
  used_bytes: z.number(),
  quota_bytes: z.number().optional(),
  file_count: z.number(),
});
export type UsageEntry = z.infer<typeof UsageEntrySchema>;

export const UsageResponseSchema = z.object({
  group_id: z.string(),
  total_used_bytes: z.number(),
  total_quota_bytes: z.number().optional(),
  users: z.array(UsageEntrySchema),
});
export type UsageResponse = z.infer<typeof UsageResponseSchema>;

// Member with user info
export const MemberWithUserSchema = z.object({
  membership: MembershipSchema,
  user: UserSchema,
});
export type MemberWithUser = z.infer<typeof MemberWithUserSchema>;