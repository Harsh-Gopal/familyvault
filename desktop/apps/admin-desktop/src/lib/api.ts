import { FamilyVaultAPI } from '@familyvault/shared';

let apiInstance: FamilyVaultAPI | null = null;

export async function getAPI(): Promise<FamilyVaultAPI> {
  if (!apiInstance) {
    const baseURL = await window.fv.getBaseURL();
    apiInstance = new FamilyVaultAPI(baseURL, () => window.fv.getToken());
  }
  return apiInstance;
}

export function resetAPI(): void {
  apiInstance = null;
}