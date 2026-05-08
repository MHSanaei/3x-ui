// Barrel re-export so callers can `import { ObjectUtil } from '@/utils'`.
// Eventually `legacy.js` will be split into smaller modules; the barrel
// keeps that refactor non-breaking.
export * from './legacy.js';
