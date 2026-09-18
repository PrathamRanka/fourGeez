export class ConnectorConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConnectorConfigurationError";
  }
}

export class ConnectorProtocolError extends Error {
  constructor(message = "AgentPay returned an invalid protocol response") {
    super(message);
    this.name = "ConnectorProtocolError";
  }
}

export class ConnectorMessageTooLargeError extends Error {
  constructor() {
    super("Message exceeds the configured connector size limit");
    this.name = "ConnectorMessageTooLargeError";
  }
}

export class ConnectorTimeoutError extends Error {
  constructor() {
    super("AgentPay cloud request timed out");
    this.name = "ConnectorTimeoutError";
  }
}

export class ConnectorAbortError extends Error {
  constructor() {
    super("AgentPay cloud request was cancelled");
    this.name = "ConnectorAbortError";
  }
}

export class ConnectorHttpError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly requestId?: string;
  readonly retryAfterSeconds?: number;

  constructor(options: {
    status: number;
    code?: string;
    requestId?: string;
    retryAfterSeconds?: number;
  }) {
    super(`AgentPay cloud request failed with HTTP ${options.status}`);
    this.name = "ConnectorHttpError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId;
    this.retryAfterSeconds = options.retryAfterSeconds;
  }
}
