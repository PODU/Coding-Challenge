/** mdpresent's library API. */
export * from './core/slide-model.js';
export * from './core/frontmatter.js';
export * from './core/splitter.js';
export * from './core/renderer.js';
export * from './client/navigation.js';
export { loadDeck, buildDeckFromSource, readSource } from './shell/pipeline.js';
export { loadTemplate, builtinTemplateDir } from './shell/template-loader.js';
export { buildStatic } from './shell/build.js';
export { startServer } from './shell/server.js';
export { scaffold } from './shell/scaffold.js';
export { exportPdf, isPdfAvailable, PdfUnavailableError } from './shell/pdf-export.js';
