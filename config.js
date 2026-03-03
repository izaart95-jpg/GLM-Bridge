// Z.AI Proxy Pool - Configuration
// All tunables in one place

module.exports = {
  // Server settings
  server: {
    port: process.env.PORT || 3001,
    host: process.env.HOST || '0.0.0.0',
  },

  // Authentication Dont HardCore it
  auth: {
    enabled: true,
    token: process.env.AUTH_TOKEN || 'Waguri',
  },

  // Client pool settings
  pool: {
    // Rotation strategy: 'lru' (least recently used), 'round-robin', 'random'
    rotationStrategy: process.env.ROTATION_STRATEGY || 'lru',

    // Rate limit cooldown in milliseconds (5 minutes default)
    rateLimitCooldown: parseInt(process.env.RATE_LIMIT_COOLDOWN) || 5 * 60 * 1000,

    // Health check interval in milliseconds
    healthCheckInterval: parseInt(process.env.HEALTH_CHECK_INTERVAL) || 30000,
  },

  // Request queue settings
  queue: {
    // Maximum number of requests in queue (0 = unlimited)
    maxSize: parseInt(process.env.QUEUE_MAX_SIZE) || 100,

    // Maximum time a request can wait in queue (milliseconds)
    maxWaitTime: parseInt(process.env.QUEUE_MAX_WAIT) || 240000,
  },

  // Timeout settings (milliseconds)
  timeouts: {
    // Default request timeout
    default: parseInt(process.env.TIMEOUT) || 300000,

    // Streaming chunk timeout (how long to wait for next chunk)
    streamingChunk: parseInt(process.env.STREAMING_CHUNK_TIMEOUT) || 120000,
  },

  // Streaming settings
  streaming: {
    // Enable streaming by default
    enabled: true,

    // Chunk mode: 'word', 'sentence', 'mutation' (DOM mutation level)
    chunkMode: process.env.STREAMING_CHUNK_MODE || 'word',

    // Minimum characters before sending a chunk (for word mode)
    minChunkSize: parseInt(process.env.STREAMING_MIN_CHUNK) || 5,

    // Debounce time for mutations (milliseconds)
    debounceTime: parseInt(process.env.STREAMING_DEBOUNCE) || 50,
  },

  // WebSocket settings
  websocket: {
    // Keepalive ping interval (milliseconds)
    pingInterval: parseInt(process.env.WS_PING_INTERVAL) || 30000,

    // Maximum reconnection attempts (browser side)
    maxReconnectAttempts: parseInt(process.env.WS_MAX_RECONNECT) || 10,
  },

  // Logging
  logging: {
    // Log level: 'debug', 'info', 'warn', 'error'
    level: process.env.LOG_LEVEL || 'info',

    // Log format: 'text', 'json'
    format: process.env.LOG_FORMAT || 'text',
  },

  // Known Z.AI models (fallback when can't detect)
  knownModels: [
    'z1',
    'z1-mini',
  ],
};
