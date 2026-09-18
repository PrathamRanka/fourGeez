package com.agentpay.verify;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Duration;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.Base64;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Verifies AgentPay request signatures and replay state. */
public final class AgentPayVerifier {
    private static final String SIGNATURE_DOMAIN = "agentpay.seller-request.v1";
    private static final Duration MAXIMUM_AGE = Duration.ofMinutes(5);
    private final byte[] secret;
    private final ReplayStore replayStore;

    /** Creates a verifier with an explicit replay store. */
    public AgentPayVerifier(byte[] secret, ReplayStore replayStore) {
        if (secret.length < 32) {
            throw new IllegalArgumentException("AgentPay verification secret must contain at least 32 bytes");
        }
        this.secret = secret.clone();
        this.replayStore = replayStore;
    }

    /** Verifies one exact raw request and claims its transaction identifier. */
    public void verify(
            String method,
            String path,
            byte[] body,
            Map<String, String> headers,
            Instant now) {
        String signatureValue = header(headers, "x-agentpay-signature");
        String timestampValue = header(headers, "x-agentpay-timestamp");
        String transactionId = header(headers, "x-agentpay-transaction-id");
        if (signatureValue == null || timestampValue == null || transactionId == null || transactionId.isBlank()) {
            throw new VerificationException("invalid_signature");
        }
        Instant timestamp;
        byte[] providedSignature;
        try {
            timestamp = Instant.parse(timestampValue);
            providedSignature = Base64.getDecoder().decode(signatureValue);
        } catch (DateTimeParseException | IllegalArgumentException exception) {
            throw new VerificationException("invalid_signature");
        }
        if (Duration.between(timestamp, now).abs().compareTo(MAXIMUM_AGE) > 0) {
            throw new VerificationException("stale_request");
        }
        String canonical = String.join(
                "\n",
                SIGNATURE_DOMAIN,
                timestampValue,
                method.toUpperCase(),
                path,
                hex(sha256(body)),
                transactionId);
        byte[] expectedSignature = hmac(secret, canonical.getBytes(StandardCharsets.UTF_8));
        if (!MessageDigest.isEqual(providedSignature, expectedSignature)) {
            throw new VerificationException("invalid_signature");
        }
        if (!replayStore.claim(transactionId, timestamp)) {
            throw new VerificationException("replay");
        }
    }

    /** Finds a header without depending on map key casing. */
    private static String header(Map<String, String> headers, String name) {
        for (Map.Entry<String, String> header : headers.entrySet()) {
            if (header.getKey().equalsIgnoreCase(name)) {
                return header.getValue();
            }
        }
        return null;
    }

    /** Calculates a SHA-256 digest. */
    private static byte[] sha256(byte[] value) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(value);
        } catch (Exception exception) {
            throw new IllegalStateException(exception);
        }
    }

    /** Calculates an HMAC-SHA256 digest. */
    private static byte[] hmac(byte[] key, byte[] value) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(key, "HmacSHA256"));
            return mac.doFinal(value);
        } catch (Exception exception) {
            throw new IllegalStateException(exception);
        }
    }

    /** Encodes lowercase hexadecimal bytes. */
    private static String hex(byte[] value) {
        StringBuilder result = new StringBuilder(value.length * 2);
        for (byte current : value) {
            result.append(String.format("%02x", current));
        }
        return result.toString();
    }

    /** Stores accepted transaction identifiers atomically. */
    public interface ReplayStore {
        /** Returns true only for the first accepted transaction identifier. */
        boolean claim(String transactionId, Instant timestamp);
    }

    /** Provides process-local replay protection for tests and development. */
    public static final class MemoryReplayStore implements ReplayStore {
        private final Set<String> transactionIds = new HashSet<>();

        /** Claims one transaction identifier exactly once. */
        @Override
        public synchronized boolean claim(String transactionId, Instant timestamp) {
            return transactionIds.add(transactionId);
        }
    }

    /** Reports a stable machine-readable verification failure. */
    public static final class VerificationException extends RuntimeException {
        /** Creates one verification failure. */
        public VerificationException(String code) {
            super(code);
        }
    }
}
