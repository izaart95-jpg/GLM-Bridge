// Z.AI Direct Bridge - Configuration
// All tunables in one place

module.exports = {
  // Server settings
  server: {
    port: process.env.PORT || 3001,
    host: process.env.HOST || '0.0.0.0',
  },

  // Authentication
  auth: {
    enabled: true,
    token: process.env.AUTH_TOKEN || 'Waguri',
  },

  // Timeout settings (milliseconds)
  timeouts: {
    // Default request timeout
    default: parseInt(process.env.TIMEOUT) || 300000,
  },

  // Z.AI Direct Token (leave empty to use guest flow)
  zaiToken: process.env.ZAI_TOKEN || '',

  // Logging
  logging: {
    // Log level: 'debug', 'info', 'warn', 'error'
    level: process.env.LOG_LEVEL || 'debug',

    // Log format: 'text', 'json'
    format: process.env.LOG_FORMAT || 'text',
  },

  // Known Z.AI models (fallback when can't detect)
  knownModels: [
    'GLM-5.1',
    'GLM-5',
  ],
};
