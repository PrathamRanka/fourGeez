package com.agentpay.verify;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;
import java.util.Map;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.security.MessageDigest;

/** Runs dependency-free known-answer verification tests. */
public final class AgentPayVerifierTest {
    /** Executes the verifier and replay assertions. */
    public static void main(String[] arguments) throws Exception {
        byte[] secret = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8);
        byte[] body = "hello".getBytes(StandardCharsets.UTF_8);
        String timestamp = "2026-09-17T10:00:00Z";
        String transactionId = "txn_java";
        Map<String, String> headers = Map.of(
                "x-agentpay-signature", sign(secret, body, timestamp, transactionId),
                "x-agentpay-timestamp", timestamp,
                "x-agentpay-transaction-id", transactionId);
        AgentPayVerifier verifier = new AgentPayVerifier(secret, new AgentPayVerifier.MemoryReplayStore());
        AgentPayVerificationFilter filter = new AgentPayVerificationFilter(verifier);
        filter.verify("POST", "/fulfill", body, headers, Instant.parse(timestamp));
        assertRejected(verifier, body, headers, Instant.parse(timestamp), "replay");

        AgentPayVerifier modifiedVerifier = new AgentPayVerifier(secret, new AgentPayVerifier.MemoryReplayStore());
        assertRejected(
                modifiedVerifier,
                "changed".getBytes(StandardCharsets.UTF_8),
                headers,
                Instant.parse(timestamp),
                "invalid_signature");

        String staleTransactionId = "txn_java_stale";
        Map<String, String> staleHeaders = Map.of(
                "x-agentpay-signature", sign(secret, body, timestamp, staleTransactionId),
                "x-agentpay-timestamp", timestamp,
                "x-agentpay-transaction-id", staleTransactionId);
        AgentPayVerifier staleVerifier = new AgentPayVerifier(secret, new AgentPayVerifier.MemoryReplayStore());
        assertRejected(
                staleVerifier,
                body,
                staleHeaders,
                Instant.parse(timestamp).plusSeconds(301),
                "stale_request");

        try {
            new AgentPayVerifier("short".getBytes(StandardCharsets.UTF_8), new AgentPayVerifier.MemoryReplayStore());
            throw new AssertionError("short secret was accepted");
        } catch (IllegalArgumentException expected) {
            // The constructor must reject weak signing secrets before any request is accepted.
        }
    }

    /** Verifies that one request fails with the expected stable error code. */
    private static void assertRejected(
            AgentPayVerifier verifier,
            byte[] body,
            Map<String, String> headers,
            Instant now,
            String expectedCode) {
        try {
            verifier.verify("POST", "/fulfill", body, headers, now);
            throw new AssertionError(expectedCode + " request was accepted");
        } catch (AgentPayVerifier.VerificationException exception) {
            if (!expectedCode.equals(exception.getMessage())) {
                throw exception;
            }
        }
    }

    /** Creates an independent known-answer request signature. */
    private static String sign(byte[] secret, byte[] body, String timestamp, String transactionId) throws Exception {
        String bodyHash = hex(MessageDigest.getInstance("SHA-256").digest(body));
        String canonical = String.join("\n", "agentpay.seller-request.v1", timestamp, "POST", "/fulfill", bodyHash, transactionId);
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(secret, "HmacSHA256"));
        return Base64.getEncoder().encodeToString(mac.doFinal(canonical.getBytes(StandardCharsets.UTF_8)));
    }

    /** Encodes lowercase hexadecimal bytes. */
    private static String hex(byte[] value) {
        StringBuilder result = new StringBuilder(value.length * 2);
        for (byte current : value) {
            result.append(String.format("%02x", current));
        }
        return result.toString();
    }
}
