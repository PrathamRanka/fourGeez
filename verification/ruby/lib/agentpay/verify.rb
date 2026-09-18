# frozen_string_literal: true

require "base64"
require "digest"
require "openssl"
require "stringio"
require "time"

module AgentPay
  SIGNATURE_DOMAIN = "agentpay.seller-request.v1"
  MAXIMUM_AGE_SECONDS = 300

  # Reports a stable machine-readable verification failure.
  class VerificationError < StandardError; end

  # Provides process-local replay protection for tests and development.
  class MemoryReplayStore
    # Creates an empty replay store.
    def initialize
      @transaction_ids = {}
      @lock = Mutex.new
    end

    # Claims one transaction identifier exactly once.
    def claim(transaction_id, timestamp)
      _ = timestamp
      @lock.synchronize do
        return false if @transaction_ids.key?(transaction_id)

        @transaction_ids[transaction_id] = true
        true
      end
    end
  end

  # Verifies exact request bytes and replay state.
  class Verifier
    # Creates a verifier with an explicit replay store.
    def initialize(secret:, replay_store:)
      raise ArgumentError, "AgentPay verification secret must contain at least 32 bytes" if secret.bytesize < 32

      @secret = secret.dup
      @replay_store = replay_store
    end

    # Verifies one request and claims its transaction identifier.
    def verify(method:, path:, body:, headers:, now: Time.now.utc)
      signature_value = header(headers, "x-agentpay-signature")
      timestamp_value = header(headers, "x-agentpay-timestamp")
      transaction_id = header(headers, "x-agentpay-transaction-id")
      raise VerificationError, "invalid_signature" if [signature_value, timestamp_value, transaction_id].any?(&:nil?)

      begin
        timestamp = Time.iso8601(timestamp_value)
        provided_signature = Base64.strict_decode64(signature_value)
      rescue ArgumentError
        raise VerificationError, "invalid_signature"
      end
      raise VerificationError, "stale_request" if (now - timestamp).abs > MAXIMUM_AGE_SECONDS

      body_hash = Digest::SHA256.hexdigest(body)
      canonical = [SIGNATURE_DOMAIN, timestamp_value, method.upcase, path, body_hash, transaction_id].join("\n")
      expected_signature = OpenSSL::HMAC.digest("SHA256", @secret, canonical)
      unless provided_signature.bytesize == expected_signature.bytesize &&
             OpenSSL.fixed_length_secure_compare(provided_signature, expected_signature)
        raise VerificationError, "invalid_signature"
      end
      raise VerificationError, "replay" unless @replay_store.claim(transaction_id, timestamp)
    end

    private

    # Finds a header without depending on hash key casing.
    def header(headers, name)
      pair = headers.find { |header_name, _value| header_name.casecmp?(name) }
      pair&.last
    end
  end

  # Verifies Rack requests before Rails controller execution.
  class VerificationMiddleware
    # Creates middleware around the downstream Rack application.
    def initialize(application, verifier:, clock: -> { Time.now.utc })
      @application = application
      @verifier = verifier
      @clock = clock
    end

    # Reads and restores rack.input before calling the application.
    def call(environment)
      body = environment.fetch("rack.input").read
      environment["rack.input"] = StringIO.new(body)
      headers = {
        "x-agentpay-signature" => environment["HTTP_X_AGENTPAY_SIGNATURE"],
        "x-agentpay-timestamp" => environment["HTTP_X_AGENTPAY_TIMESTAMP"],
        "x-agentpay-transaction-id" => environment["HTTP_X_AGENTPAY_TRANSACTION_ID"]
      }
      @verifier.verify(
        method: environment.fetch("REQUEST_METHOD"),
        path: environment.fetch("PATH_INFO"),
        body: body,
        headers: headers,
        now: @clock.call
      )
      @application.call(environment)
    rescue VerificationError => error
      reject(error.message == "replay" ? 409 : 401)
    rescue StandardError
      reject(503)
    end

    private

    # Returns the shared redacted Rack error response.
    def reject(status)
      [status, { "content-type" => "application/json" }, ['{"error":"AgentPay verification failed"}']]
    end
  end
end
