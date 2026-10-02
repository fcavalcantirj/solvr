import { createRequire } from "module";

const require = createRequire(import.meta.url);

/** The CLI's version: the published package version. */
export const VERSION: string = require("../package.json").version;
