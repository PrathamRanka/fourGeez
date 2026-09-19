import assert from "node:assert/strict";
import { test } from "node:test";

import {
  buildVercelEnvironment,
  validateDeploymentOutputs,
} from "./vercel-environment.mjs";

const outputs = {
  environment: { value: "dev" },
  web_origin: { value: "https://agentpay.prathamranka.in" },
  http_api_url: { value: "https://abc.execute-api.ap-south-1.amazonaws.com" },
  seller_user_pool_client_id: { value: "client123" },
  aws_region: { value: "ap-south-1" },
};

test("Vercel environment uses only non-secret Terraform outputs", () => {
  validateDeploymentOutputs(outputs);
  assert.deepEqual(buildVercelEnvironment(outputs), {
    AGENTPAY_ENV: "dev",
    AGENTPAY_IDENTITY_MODE: "cognito",
    AGENTPAY_WEB_ORIGIN: "https://agentpay.prathamranka.in",
    AGENTPAY_API_ORIGIN:
      "https://abc.execute-api.ap-south-1.amazonaws.com",
    AWS_REGION: "ap-south-1",
    AGENTPAY_SELLER_USER_POOL_CLIENT_ID: "client123",
  });
});

test("Vercel environment rejects a disabled API or an unapproved origin", () => {
  assert.throws(
    () =>
      validateDeploymentOutputs({
        ...outputs,
        http_api_url: { value: null },
      }),
    /API deployment is disabled/,
  );
  assert.throws(
    () =>
      validateDeploymentOutputs({
        ...outputs,
        web_origin: { value: "https://preview.example" },
      }),
    /canonical HTTPS web origin/,
  );
});
