# frozen_string_literal: true

require "minitest/autorun"
require_relative "../lib/agentpay/verify"

# Verifies known-answer signatures and replay behavior.
class AgentPayVerifierTest < Minitest::Test
  SECRET = "0123456789abcdef0123456789abcdef"
  TIMESTAMP = "2026-09-17T10:00:00Z"

  # Verifies a valid request and rejects its replay.
  def test_verifies_and_blocks_replay
    headers = signed_headers("hello", TIMESTAMP, "txn_ruby")
    verifier = AgentPay::Verifier.new(secret: SECRET, replay_store: AgentPay::MemoryReplayStore.new)
    verifier.verify(method: "POST", path: "/fulfill", body: "hello", headers: headers, now: Time.iso8601(TIMESTAMP))
    error = assert_raises(AgentPay::VerificationError) do
      verifier.verify(method: "POST", path: "/fulfill", body: "hello", headers: headers, now: Time.iso8601(TIMESTAMP))
    end
    assert_equal "replay", error.message
  end

  # Rejects a body that differs from the signed bytes.
  def test_rejects_modified_body
    verifier = AgentPay::Verifier.new(secret: SECRET, replay_store: AgentPay::MemoryReplayStore.new)
    error = assert_raises(AgentPay::VerificationError) do
      verifier.verify(
        method: "POST",
        path: "/fulfill",
        body: "changed",
        headers: signed_headers("hello", TIMESTAMP, "txn_ruby_changed"),
        now: Time.iso8601(TIMESTAMP)
      )
    end
    assert_equal "invalid_signature", error.message
  end

  # Rejects a correctly signed request outside the freshness window.
  def test_rejects_stale_request
    verifier = AgentPay::Verifier.new(secret: SECRET, replay_store: AgentPay::MemoryReplayStore.new)
    error = assert_raises(AgentPay::VerificationError) do
      verifier.verify(
        method: "POST",
        path: "/fulfill",
        body: "hello",
        headers: signed_headers("hello", TIMESTAMP, "txn_ruby_stale"),
        now: Time.iso8601(TIMESTAMP) + 301
      )
    end
    assert_equal "stale_request", error.message
  end

  # Rejects secrets shorter than the documented minimum.
  def test_rejects_short_secret
    assert_raises(ArgumentError) do
      AgentPay::Verifier.new(secret: "short", replay_store: AgentPay::MemoryReplayStore.new)
    end
  end

  # Preserves the Rack request body for the downstream Rails application.
  def test_middleware_restores_request_body
    captured_body = nil
    application = lambda do |environment|
      captured_body = environment.fetch("rack.input").read
      [204, {}, []]
    end
    verifier = AgentPay::Verifier.new(secret: SECRET, replay_store: AgentPay::MemoryReplayStore.new)
    middleware = AgentPay::VerificationMiddleware.new(
      application,
      verifier: verifier,
      clock: -> { Time.iso8601(TIMESTAMP) }
    )
    environment = {
      "rack.input" => StringIO.new("hello"),
      "REQUEST_METHOD" => "POST",
      "PATH_INFO" => "/fulfill",
      "HTTP_X_AGENTPAY_SIGNATURE" => sign("hello", TIMESTAMP, "txn_ruby_middleware"),
      "HTTP_X_AGENTPAY_TIMESTAMP" => TIMESTAMP,
      "HTTP_X_AGENTPAY_TRANSACTION_ID" => "txn_ruby_middleware"
    }

    response = middleware.call(environment)

    assert_equal 204, response.first
    assert_equal "hello", captured_body
  end

  private

  # Creates an independent known-answer request signature.
  def sign(body, timestamp, transaction_id)
    body_hash = Digest::SHA256.hexdigest(body)
    canonical = [AgentPay::SIGNATURE_DOMAIN, timestamp, "POST", "/fulfill", body_hash, transaction_id].join("\n")
    Base64.strict_encode64(OpenSSL::HMAC.digest("SHA256", SECRET, canonical))
  end

  # Creates the complete signed request header set.
  def signed_headers(body, timestamp, transaction_id)
    {
      "x-agentpay-signature" => sign(body, timestamp, transaction_id),
      "x-agentpay-timestamp" => timestamp,
      "x-agentpay-transaction-id" => transaction_id
    }
  end
end
