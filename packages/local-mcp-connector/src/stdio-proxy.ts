import { once } from "node:events";
import type { Readable, Writable } from "node:stream";

import {
  DEFAULT_MAX_MESSAGE_BYTES,
  MAXIMUM_MCP_MESSAGE_BYTES,
  validatePositiveInteger,
} from "./http-boundaries.js";
import type { McpTransport } from "./cloud-transport.js";

const JSON_RPC_PARSE_ERROR = -32700;
const JSON_RPC_INVALID_REQUEST = -32600;
const JSON_RPC_CLOUD_ERROR = -32000;

export interface StdioProxyOptions {
  input: Readable;
  output: Writable;
  diagnostics: Writable;
  transport: McpTransport;
  maxMessageBytes?: number;
  signal?: AbortSignal;
}

export async function runStdioProxy(options: StdioProxyOptions): Promise<void> {
  const maximumBytes = validatePositiveInteger(
    options.maxMessageBytes ?? DEFAULT_MAX_MESSAGE_BYTES,
    "maxMessageBytes",
    MAXIMUM_MCP_MESSAGE_BYTES,
  );
  let chunks: Buffer[] = [];
  let bufferedBytes = 0;
  let discardingOversizedLine = false;

  for await (const rawChunk of options.input) {
    if (options.signal?.aborted) {
      break;
    }
    const chunk = Buffer.isBuffer(rawChunk) ? rawChunk : Buffer.from(rawChunk);
    let cursor = 0;
    while (cursor < chunk.byteLength) {
      const newlineIndex = chunk.indexOf(0x0a, cursor);
      const end = newlineIndex === -1 ? chunk.byteLength : newlineIndex;
      const segment = chunk.subarray(cursor, end);
      if (!discardingOversizedLine) {
        if (bufferedBytes + segment.byteLength > maximumBytes) {
          chunks = [];
          bufferedBytes = 0;
          discardingOversizedLine = true;
        } else if (segment.byteLength > 0) {
          chunks.push(segment);
          bufferedBytes += segment.byteLength;
        }
      }

      if (newlineIndex === -1) {
        break;
      }
      if (discardingOversizedLine) {
        await writeJsonRpcError(
          options.output,
          null,
          JSON_RPC_INVALID_REQUEST,
          "Request exceeds connector size limit",
        );
      } else {
        await processLine(Buffer.concat(chunks, bufferedBytes), options);
      }
      chunks = [];
      bufferedBytes = 0;
      discardingOversizedLine = false;
      cursor = newlineIndex + 1;
    }
  }

  if (discardingOversizedLine) {
    await writeJsonRpcError(
      options.output,
      null,
      JSON_RPC_INVALID_REQUEST,
      "Request exceeds connector size limit",
    );
  } else if (bufferedBytes > 0 && !options.signal?.aborted) {
    await processLine(Buffer.concat(chunks, bufferedBytes), options);
  }
}

async function processLine(
  lineBuffer: Buffer,
  options: StdioProxyOptions,
): Promise<void> {
  const line = stripTrailingCarriageReturn(lineBuffer).toString("utf8");
  if (line.length === 0) {
    return;
  }

  let request: unknown;
  try {
    request = JSON.parse(line);
  } catch {
    await writeJsonRpcError(
      options.output,
      null,
      JSON_RPC_PARSE_ERROR,
      "Parse error",
    );
    return;
  }

  const requestId = extractRequestId(request);
  try {
    const response = await options.transport.send(line, options.signal);
    if (response !== undefined) {
      for (const responseLine of response.split("\n")) {
        await writeLine(options.output, responseLine);
      }
    }
  } catch (error) {
    await writeSafeDiagnostic(options.diagnostics, error);
    await writeJsonRpcError(
      options.output,
      requestId,
      JSON_RPC_CLOUD_ERROR,
      "AgentPay cloud request failed",
    );
  }
}

function stripTrailingCarriageReturn(line: Buffer): Buffer {
  return line.byteLength > 0 && line[line.byteLength - 1] === 0x0d
    ? line.subarray(0, line.byteLength - 1)
    : line;
}

function extractRequestId(request: unknown): string | number | null {
  if (
    typeof request !== "object" ||
    request === null ||
    Array.isArray(request)
  ) {
    return null;
  }
  const requestId = (request as Record<string, unknown>).id;
  return typeof requestId === "string" || typeof requestId === "number"
    ? requestId
    : null;
}

async function writeJsonRpcError(
  output: Writable,
  id: string | number | null,
  code: number,
  message: string,
): Promise<void> {
  await writeLine(
    output,
    JSON.stringify({ jsonrpc: "2.0", id, error: { code, message } }),
  );
}

async function writeSafeDiagnostic(
  diagnostics: Writable,
  error: unknown,
): Promise<void> {
  const metadata = isRecord(error) ? error : {};
  const status =
    typeof metadata.status === "number" ? metadata.status : undefined;
  const code = typeof metadata.code === "string" ? metadata.code : undefined;
  const requestId =
    typeof metadata.requestId === "string" ? metadata.requestId : undefined;
  const fields = [
    "AgentPay connector request failed",
    status === undefined ? undefined : `status=${status}`,
    code === undefined ? undefined : `code=${safeDiagnosticValue(code)}`,
    requestId === undefined
      ? undefined
      : `requestId=${safeDiagnosticValue(requestId)}`,
  ].filter((field): field is string => field !== undefined);
  await writeLine(diagnostics, fields.join(" "));
}

function safeDiagnosticValue(value: string): string {
  return /^[A-Za-z0-9_.:-]{1,128}$/u.test(value) ? value : "redacted";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

async function writeLine(stream: Writable, line: string): Promise<void> {
  if (!stream.write(`${line}\n`)) {
    await once(stream, "drain");
  }
}
