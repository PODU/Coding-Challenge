import { Router } from 'express';
import { asyncHandler, getUserId, parse, requireAuth } from '../http.js';
import type { Services } from '../http.js';
import { loginSchema, registerSchema } from '../domain/schemas.js';

export function authRouter(services: Services): Router {
  const router = Router();

  router.post(
    '/register',
    asyncHandler((req, res) => {
      const { email, password } = parse(registerSchema, req.body);
      res.status(201).json(services.auth.register(email, password));
    }),
  );

  router.post(
    '/login',
    asyncHandler((req, res) => {
      const { email, password } = parse(loginSchema, req.body);
      res.status(200).json(services.auth.login(email, password));
    }),
  );

  router.get(
    '/me',
    requireAuth(services.auth),
    asyncHandler((req, res) => {
      res.json({ user: services.auth.getUser(getUserId(req)) });
    }),
  );

  return router;
}
