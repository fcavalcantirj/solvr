import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);

/** The server's version: the published package version. */
export const VERSION: string = require('../package.json').version;
