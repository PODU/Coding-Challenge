/**
 * Points by asteroid tier. Smaller rocks award more.
 */

import { ASTEROID_SCORE } from "./constants.js";
import type { AsteroidTier } from "./entities.js";

export const pointsFor = (tier: AsteroidTier): number => ASTEROID_SCORE[tier];
