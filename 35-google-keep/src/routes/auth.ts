import { Router } from 'express';
import { z } from 'zod';
import type { Config } from '../config';
import type { GoogleAuthClient } from '../auth/google';
import { newState } from '../auth/google';
import { asyncHandler, unauthenticated, validationError, HttpError } from '../middleware/errors';
import { clearSession, setSession } from '../session';

const localSchema = z.object({ username: z.string().trim().min(1).max(100) });

const OAUTH_STATE_COOKIE = 'keep_oauth_state';

export interface AuthDeps {
  config: Config;
  google: GoogleAuthClient | null;
}

export function authRouter(deps: AuthDeps): Router {
  const router = Router();
  const { config } = deps;

  // Current user.
  router.get(
    '/me',
    asyncHandler((req, res) => {
      if (!req.userId) throw unauthenticated();
      const user = req.repos!.users.getById(req.userId);
      if (!user) throw unauthenticated();
      res.json({
        user: {
          id: user.id,
          email: user.email,
          displayName: user.displayName,
          provider: user.provider,
        },
      });
    }),
  );

  // Dev-only local login.
  router.post(
    '/local',
    asyncHandler((req, res) => {
      if (!config.authLocal) throw new HttpError(404, 'NOT_FOUND', 'Local auth is disabled');
      const { username } = localSchema.parse(req.body ?? {});
      const user = req.repos!.users.upsertFromOAuth({
        provider: 'local',
        providerId: username.toLowerCase(),
        displayName: username,
        email: null,
      });
      setSession(res, user.id, config.sessionSecret);
      res.json({
        user: { id: user.id, displayName: user.displayName, provider: user.provider },
      });
    }),
  );

  // Begin Google OAuth.
  router.get(
    '/google',
    asyncHandler((req, res) => {
      if (!deps.google) throw new HttpError(404, 'NOT_FOUND', 'Google auth is not configured');
      const state = newState();
      res.append(
        'Set-Cookie',
        `${OAUTH_STATE_COOKIE}=${state}; Path=/; HttpOnly; SameSite=Lax; Max-Age=600`,
      );
      const redirectUri = `${req.protocol}://${req.get('host')}/api/auth/google/callback`;
      res.redirect(deps.google.getAuthUrl(state, redirectUri));
    }),
  );

  // Google OAuth callback.
  router.get(
    '/google/callback',
    asyncHandler(async (req, res) => {
      if (!deps.google) throw new HttpError(404, 'NOT_FOUND', 'Google auth is not configured');
      const code = typeof req.query.code === 'string' ? req.query.code : '';
      const state = typeof req.query.state === 'string' ? req.query.state : '';
      if (!code) throw validationError('missing authorization code');

      const cookieState = (req.headers.cookie ?? '')
        .split(';')
        .map((c) => c.trim())
        .find((c) => c.startsWith(`${OAUTH_STATE_COOKIE}=`))
        ?.split('=')[1];
      if (!cookieState || cookieState !== state) {
        throw validationError('invalid OAuth state');
      }

      const redirectUri = `${req.protocol}://${req.get('host')}/api/auth/google/callback`;
      const profile = await deps.google.exchangeCode(code, redirectUri);
      const user = req.repos!.users.upsertFromOAuth({
        provider: 'google',
        providerId: profile.providerId,
        email: profile.email,
        displayName: profile.displayName,
      });
      setSession(res, user.id, config.sessionSecret);
      res.append('Set-Cookie', `${OAUTH_STATE_COOKIE}=; Path=/; HttpOnly; Max-Age=0`);
      res.redirect('/');
    }),
  );

  router.post(
    '/logout',
    asyncHandler((_req, res) => {
      clearSession(res);
      res.json({ ok: true });
    }),
  );

  return router;
}
