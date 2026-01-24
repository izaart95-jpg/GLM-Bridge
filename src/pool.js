// Client Pool Manager for Z.AI
// Handles multiple browser clients with LRU rotation, rate limiting, and health tracking

const config = require('../config');

class ClientPool {
  constructor() {
    // Map of clientId -> client state
    this.clients = new Map();

    // Request queue
    this.queue = [];

    // Currently processing requests (clientId -> request)
    this.activeRequests = new Map();

    // Stats
    this.stats = {
      totalRequests: 0,
      successfulRequests: 0,
      failedRequests: 0,
      rateLimitHits: 0,
      queuedRequests: 0,
      averageLatency: 0,
      latencySum: 0,
    };
  }

  /**
   * Register a new client
   */
  addClient(clientId, ws, ip) {
    this.clients.set(clientId, {
      ws,
      ip,
      status: 'connecting', // 'connecting', 'idle', 'busy', 'rate-limited', 'unhealthy'
      connectedAt: Date.now(),
      lastUsed: 0,
      requestCount: 0,
      rateLimitedAt: null,
      rateLimitMessage: null,
      ready: false,
      currentModel: 'z1',
      features: {
        search: false,
        deepThink: false,
      },
      health: {
        lastPing: Date.now(),
        lastPong: null,
        missedPongs: 0,
      },
    });

    console.log(`[Pool] Client added: ${clientId} from ${ip}`);
    return this.clients.get(clientId);
  }

  /**
   * Mark client as ready
   */
  setClientReady(clientId) {
    const client = this.clients.get(clientId);
    if (client) {
      client.ready = true;
      client.status = 'idle';
      console.log(`[Pool] Client ready: ${clientId}`);
      this.processQueue();
    }
  }

  /**
   * Remove a client
   */
  removeClient(clientId) {
    const client = this.clients.get(clientId);
    if (client) {
      // If client had an active request, re-queue it or fail it
      const activeRequest = this.activeRequests.get(clientId);
      if (activeRequest) {
        activeRequest.reject(new Error('Client disconnected'));
        this.activeRequests.delete(clientId);
      }
      this.clients.delete(clientId);
      console.log(`[Pool] Client removed: ${clientId}`);
    }
  }

  /**
   * Update client model info
   */
  updateClientModel(clientId, model) {
    const client = this.clients.get(clientId);
    if (client) {
      client.currentModel = model;
    }
  }

  /**
   * Update client features
   */
  updateClientFeatures(clientId, features) {
    const client = this.clients.get(clientId);
    if (client) {
      Object.assign(client.features, features);
    }
  }

  /**
   * Mark client as rate limited
   */
  setClientRateLimited(clientId, message) {
    const client = this.clients.get(clientId);
    if (client) {
      client.status = 'rate-limited';
      client.rateLimitedAt = Date.now();
      client.rateLimitMessage = message;
      this.stats.rateLimitHits++;
      console.log(`[Pool] Client rate-limited: ${clientId}`);

      // Schedule recovery check
      setTimeout(() => this.checkRateLimitRecovery(clientId), config.pool.rateLimitCooldown);
    }
  }

  /**
   * Check if rate-limited client can recover
   */
  checkRateLimitRecovery(clientId) {
    const client = this.clients.get(clientId);
    if (client && client.status === 'rate-limited') {
      const elapsed = Date.now() - client.rateLimitedAt;
      if (elapsed >= config.pool.rateLimitCooldown) {
        client.status = 'idle';
        client.rateLimitedAt = null;
        client.rateLimitMessage = null;
        console.log(`[Pool] Client recovered from rate limit: ${clientId}`);
        this.processQueue();
      }
    }
  }

  /**
   * Get an available client using configured rotation strategy
   */
  getAvailableClient(preferredClientId = null) {
    // If preferred client specified and available, use it (for session affinity)
    if (preferredClientId) {
      const preferred = this.clients.get(preferredClientId);
      if (preferred && preferred.ready && preferred.status === 'idle') {
        return preferredClientId;
      }
    }

    // Get all idle clients
    const idleClients = [];
    for (const [id, client] of this.clients) {
      if (client.ready && client.status === 'idle') {
        idleClients.push({ id, client });
      }
    }

    if (idleClients.length === 0) {
      return null;
    }

    // Apply rotation strategy
    switch (config.pool.rotationStrategy) {
      case 'lru':
        // Least recently used - pick client with oldest lastUsed
        idleClients.sort((a, b) => a.client.lastUsed - b.client.lastUsed);
        return idleClients[0].id;

      case 'round-robin':
        // Simple round-robin - pick client with lowest request count
        idleClients.sort((a, b) => a.client.requestCount - b.client.requestCount);
        return idleClients[0].id;

      case 'random':
        // Random selection
        const idx = Math.floor(Math.random() * idleClients.length);
        return idleClients[idx].id;

      default:
        return idleClients[0].id;
    }
  }

  /**
   * Mark client as busy with a request
   */
  setClientBusy(clientId, requestId) {
    const client = this.clients.get(clientId);
    if (client) {
      client.status = 'busy';
      client.lastUsed = Date.now();
      client.requestCount++;
    }
  }

  /**
   * Mark client as idle (request complete)
   */
  setClientIdle(clientId) {
    const client = this.clients.get(clientId);
    if (client && client.status === 'busy') {
      client.status = 'idle';
      this.activeRequests.delete(clientId);
      this.processQueue();
    }
  }

  /**
   * Queue a request
   * Returns a promise that resolves when a client is assigned
   */
  queueRequest(request) {
    return new Promise((resolve, reject) => {
      const queueEntry = {
        request,
        resolve,
        reject,
        queuedAt: Date.now(),
        timeoutId: null,
      };

      // Check queue size limit
      if (config.queue.maxSize > 0 && this.queue.length >= config.queue.maxSize) {
        reject(new Error('Queue is full'));
        return;
      }

      // Set timeout
      queueEntry.timeoutId = setTimeout(() => {
        const idx = this.queue.indexOf(queueEntry);
        if (idx !== -1) {
          this.queue.splice(idx, 1);
          reject(new Error('Queue timeout'));
        }
      }, config.queue.maxWaitTime);

      this.queue.push(queueEntry);
      this.stats.queuedRequests++;

      console.log(`[Pool] Request queued. Queue size: ${this.queue.length}`);

      // Try to process immediately
      this.processQueue();
    });
  }

  /**
   * Process queued requests
   */
  processQueue() {
    while (this.queue.length > 0) {
      const availableClientId = this.getAvailableClient();
      if (!availableClientId) {
        break; // No available clients
      }

      const queueEntry = this.queue.shift();
      clearTimeout(queueEntry.timeoutId);

      const client = this.clients.get(availableClientId);
      this.setClientBusy(availableClientId, queueEntry.request.id);

      queueEntry.resolve({
        clientId: availableClientId,
        client,
        queueTime: Date.now() - queueEntry.queuedAt,
      });
    }
  }

  /**
   * Send a request to a specific client
   */
  async sendRequest(clientId, request) {
    const client = this.clients.get(clientId);
    if (!client || !client.ws) {
      throw new Error('Client not found');
    }

    this.stats.totalRequests++;
    const startTime = Date.now();

    return new Promise((resolve, reject) => {
      // Store request handler
      this.activeRequests.set(clientId, {
        resolve: (result) => {
          const latency = Date.now() - startTime;
          this.stats.successfulRequests++;
          this.stats.latencySum += latency;
          this.stats.averageLatency = this.stats.latencySum / this.stats.successfulRequests;
          this.setClientIdle(clientId);
          resolve(result);
        },
        reject: (error) => {
          this.stats.failedRequests++;
          this.setClientIdle(clientId);
          reject(error);
        },
        request,
        startTime,
      });

      // Send to browser
      client.ws.send(JSON.stringify(request));
    });
  }

  /**
   * Handle response from client
   */
  handleResponse(clientId, response) {
    const activeRequest = this.activeRequests.get(clientId);
    if (activeRequest) {
      activeRequest.resolve(response);
    }
  }

  /**
   * Handle error from client
   */
  handleError(clientId, error) {
    const activeRequest = this.activeRequests.get(clientId);
    if (activeRequest) {
      activeRequest.reject(new Error(error));
    }
  }

  /**
   * Handle rate limit from client
   */
  handleRateLimit(clientId, message) {
    this.setClientRateLimited(clientId, message);
    const activeRequest = this.activeRequests.get(clientId);
    if (activeRequest) {
      activeRequest.reject(new Error('Rate limited: ' + message));
    }
  }

  /**
   * Record ping sent
   */
  recordPing(clientId) {
    const client = this.clients.get(clientId);
    if (client) {
      client.health.lastPing = Date.now();
    }
  }

  /**
   * Record pong received
   */
  recordPong(clientId) {
    const client = this.clients.get(clientId);
    if (client) {
      client.health.lastPong = Date.now();
      client.health.missedPongs = 0;
    }
  }

  /**
   * Check health of all clients
   */
  checkHealth() {
    for (const [clientId, client] of this.clients) {
      if (client.health.lastPing && !client.health.lastPong) {
        client.health.missedPongs++;
      } else if (client.health.lastPong && client.health.lastPing > client.health.lastPong) {
        client.health.missedPongs++;
      }

      if (client.health.missedPongs >= 3) {
        client.status = 'unhealthy';
        console.log(`[Pool] Client unhealthy (missed ${client.health.missedPongs} pongs): ${clientId}`);
      }
    }
  }

  /**
   * Get pool status
   */
  getStatus() {
    const clientList = [];
    let idleCount = 0;
    let busyCount = 0;
    let rateLimitedCount = 0;
    let unhealthyCount = 0;

    for (const [id, client] of this.clients) {
      clientList.push({
        id,
        ip: client.ip,
        status: client.status,
        ready: client.ready,
        connectedAt: client.connectedAt,
        lastUsed: client.lastUsed,
        requestCount: client.requestCount,
        currentModel: client.currentModel,
        features: client.features,
        rateLimitedAt: client.rateLimitedAt,
      });

      switch (client.status) {
        case 'idle': idleCount++; break;
        case 'busy': busyCount++; break;
        case 'rate-limited': rateLimitedCount++; break;
        case 'unhealthy': unhealthyCount++; break;
      }
    }

    return {
      totalClients: this.clients.size,
      idleClients: idleCount,
      busyClients: busyCount,
      rateLimitedClients: rateLimitedCount,
      unhealthyClients: unhealthyCount,
      queueLength: this.queue.length,
      clients: clientList,
      stats: this.stats,
    };
  }

  /**
   * Get a single client's info
   */
  getClient(clientId) {
    return this.clients.get(clientId);
  }

  /**
   * Get all clients
   */
  getAllClients() {
    return this.clients;
  }
}

module.exports = ClientPool;
