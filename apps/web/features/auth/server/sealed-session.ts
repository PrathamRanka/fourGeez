import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";
import { deflateRawSync, inflateRawSync } from "node:zlib";
import type { IdentityAuthentication } from "@/features/auth/model";

const sealedSessionVersion = "aps2";
const legacySealedSessionVersion = "aps1";
const initializationVectorBytes = 12;
const authenticationTagBytes = 16;
const maximumSessionPlaintextBytes = 64 * 1_024;
const maximumSealedSessionCookieValueBytes = 3_800;

function sessionAdditionalData(version: string): Buffer {
  return Buffer.from(
    version === sealedSessionVersion
      ? "agentpay-seller-session-v2"
      : "agentpay-seller-session-v1",
    "utf8",
  );
}

function encodeAuthentication(authentication: IdentityAuthentication): Buffer {
  const plaintext = Buffer.from(JSON.stringify(authentication), "utf8");
  if (plaintext.length > maximumSessionPlaintextBytes) {
    throw new Error("Seller session payload exceeds the allowed size.");
  }
  return deflateRawSync(plaintext, { level: 9 });
}

function decodeAuthentication(version: string, plaintext: Buffer): unknown {
  const decoded =
    version === sealedSessionVersion
      ? inflateRawSync(plaintext, {
          maxOutputLength: maximumSessionPlaintextBytes,
        })
      : plaintext;
  return JSON.parse(decoded.toString("utf8"));
}

function decodeKey(encodedKey: string): Buffer {
  const key = Buffer.from(encodedKey, "base64url");
  if (key.length !== 32) {
    throw new Error(
      "AGENTPAY_SESSION_ENCRYPTION_KEY must be a base64url-encoded 32-byte key.",
    );
  }
  return key;
}

function validAuthentication(
  value: unknown,
  now: number,
): value is IdentityAuthentication {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const authentication = value as IdentityAuthentication;
  return (
    typeof authentication.accessToken === "string" &&
    authentication.accessToken.length > 0 &&
    typeof authentication.expiresAt === "string" &&
    Number.isFinite(Date.parse(authentication.expiresAt)) &&
    Date.parse(authentication.sessionExpiresAt ?? authentication.expiresAt) >
      now &&
    typeof authentication.principal?.subject === "string" &&
    authentication.principal.subject.length > 0 &&
    typeof authentication.principal.email === "string" &&
    authentication.principal.email.length > 0 &&
    typeof authentication.principal.name === "string" &&
    typeof authentication.principal.onboardingComplete === "boolean" &&
    (authentication.principal.sellerId === null ||
      typeof authentication.principal.sellerId === "string")
  );
}

export function sealSellerSession(
  authentication: IdentityAuthentication,
  encodedKey: string,
): string {
  const key = decodeKey(encodedKey);
  const initializationVector = randomBytes(initializationVectorBytes);
  const cipher = createCipheriv("aes-256-gcm", key, initializationVector);
  cipher.setAAD(sessionAdditionalData(sealedSessionVersion));
  const encodedAuthentication = encodeAuthentication(authentication);
  const ciphertext = Buffer.concat([
    cipher.update(encodedAuthentication),
    cipher.final(),
  ]);
  const authenticationTag = cipher.getAuthTag();
  const sealed = [
    sealedSessionVersion,
    initializationVector.toString("base64url"),
    ciphertext.toString("base64url"),
    authenticationTag.toString("base64url"),
  ].join(".");
  if (
    Buffer.byteLength(sealed, "utf8") > maximumSealedSessionCookieValueBytes
  ) {
    throw new Error(
      "Seller session cookie exceeds the safe browser storage limit.",
    );
  }
  return sealed;
}

export function openSellerSession(
  sealed: string,
  encodedKey: string,
  now = Date.now(),
): IdentityAuthentication | null {
  try {
    const [version, encodedVector, encodedCiphertext, encodedTag, extra] =
      sealed.split(".");
    if (
      (version !== sealedSessionVersion &&
        version !== legacySealedSessionVersion) ||
      !encodedVector ||
      !encodedCiphertext ||
      !encodedTag ||
      extra
    ) {
      return null;
    }
    const initializationVector = Buffer.from(encodedVector, "base64url");
    const authenticationTag = Buffer.from(encodedTag, "base64url");
    if (
      initializationVector.length !== initializationVectorBytes ||
      authenticationTag.length !== authenticationTagBytes
    ) {
      return null;
    }
    const decipher = createDecipheriv(
      "aes-256-gcm",
      decodeKey(encodedKey),
      initializationVector,
    );
    decipher.setAAD(sessionAdditionalData(version));
    decipher.setAuthTag(authenticationTag);
    const plaintext = Buffer.concat([
      decipher.update(Buffer.from(encodedCiphertext, "base64url")),
      decipher.final(),
    ]);
    const authentication = decodeAuthentication(version, plaintext);
    return validAuthentication(authentication, now) ? authentication : null;
  } catch {
    return null;
  }
}
