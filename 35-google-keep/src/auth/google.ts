import { randomBytes } from 'node:crypto';

export interface GoogleProfile {
  providerId: string; // Google 'sub'
  email: string | null;
  displayName: string | null;
}

/**
 * The Google OAuth 2.0 flow. Tests swap in a fake so they need no network.
 */
export interface GoogleAuthClient {
  /** Build the consent-screen redirect URL for a given state + callback. */
  getAuthUrl(state: string, redirectUri: string): string;
  /** Exchange an authorization code for the user's profile. */
  exchangeCode(code: string, redirectUri: string): Promise<GoogleProfile>;
}

export function newState(): string {
  return randomBytes(16).toString('hex');
}

export function createGoogleAuthClient(clientId: string, clientSecret: string): GoogleAuthClient {
  return {
    getAuthUrl(state, redirectUri) {
      const params = new URLSearchParams({
        client_id: clientId,
        redirect_uri: redirectUri,
        response_type: 'code',
        scope: 'openid email profile',
        state,
        access_type: 'offline',
        prompt: 'consent',
      });
      return `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
    },
    async exchangeCode(code, redirectUri) {
      const tokenRes = await fetch('https://oauth2.googleapis.com/token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
          code,
          client_id: clientId,
          client_secret: clientSecret,
          redirect_uri: redirectUri,
          grant_type: 'authorization_code',
        }),
      });
      if (!tokenRes.ok) throw new Error(`Google token exchange failed: ${tokenRes.status}`);
      const token = (await tokenRes.json()) as { access_token?: string };
      if (!token.access_token) throw new Error('Google token response missing access_token');

      const infoRes = await fetch('https://openidconnect.googleapis.com/v1/userinfo', {
        headers: { Authorization: `Bearer ${token.access_token}` },
      });
      if (!infoRes.ok) throw new Error(`Google userinfo failed: ${infoRes.status}`);
      const info = (await infoRes.json()) as {
        sub: string;
        email?: string;
        name?: string;
      };
      return {
        providerId: info.sub,
        email: info.email ?? null,
        displayName: info.name ?? null,
      };
    },
  };
}
