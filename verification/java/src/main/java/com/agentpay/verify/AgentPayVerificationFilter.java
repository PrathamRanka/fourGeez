package com.agentpay.verify;

import java.time.Instant;
import java.util.Map;

/** Provides the verification step used by a Spring OncePerRequestFilter. */
public final class AgentPayVerificationFilter {
    private final AgentPayVerifier verifier;

    /** Creates the Spring filter helper. */
    public AgentPayVerificationFilter(AgentPayVerifier verifier) {
        this.verifier = verifier;
    }

    /** Verifies buffered request bytes before the controller chain continues. */
    public void verify(String method, String path, byte[] body, Map<String, String> headers, Instant now) {
        verifier.verify(method, path, body, headers, now);
    }
}
