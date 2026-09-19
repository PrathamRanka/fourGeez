import { randomBytes } from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const canonicalWebOrigin = "https://agentpay.prathamranka.in";

function outputValue(outputs, name) {
  return outputs?.[name]?.value;
}

export function validateDeploymentOutputs(outputs) {
  const environment = outputValue(outputs, "environment");
  const webOrigin = outputValue(outputs, "web_origin");
  const apiOrigin = outputValue(outputs, "http_api_url");
  const region = outputValue(outputs, "aws_region");
  const clientID = outputValue(outputs, "seller_user_pool_client_id");

  if (!apiOrigin) {
    throw new Error("API deployment is disabled; no HTTP API output is available.");
  }
  for (const [name, value] of Object.entries({ webOrigin, apiOrigin })) {
    let parsed;
    try {
      parsed = new URL(value);
    } catch {
      throw new Error(`${name} must be an absolute HTTPS origin.`);
    }
    if (parsed.protocol !== "https:" || parsed.pathname !== "/") {
      throw new Error(`${name} must be an absolute HTTPS origin.`);
    }
  }
  if (webOrigin !== canonicalWebOrigin) {
    throw new Error(`web_origin must be the canonical HTTPS web origin ${canonicalWebOrigin}.`);
  }
  if (!["dev", "demo", "prod"].includes(environment)) {
    throw new Error("Terraform environment output is invalid.");
  }
  if (region !== "ap-south-1") {
    throw new Error("AWS region must remain ap-south-1 for the India deployment.");
  }
  if (typeof clientID !== "string" || clientID.trim() === "") {
    throw new Error("Cognito seller user-pool client output is missing.");
  }
}

export function buildVercelEnvironment(outputs) {
  validateDeploymentOutputs(outputs);
  if (outputs?.vercel_environment?.value) {
    return outputs.vercel_environment.value;
  }
  return {
    AGENTPAY_ENV: outputValue(outputs, "environment"),
    AGENTPAY_IDENTITY_MODE: "cognito",
    AGENTPAY_WEB_ORIGIN: outputValue(outputs, "web_origin"),
    AGENTPAY_API_ORIGIN: outputValue(outputs, "http_api_url"),
    AWS_REGION: outputValue(outputs, "aws_region"),
    AGENTPAY_SELLER_USER_POOL_CLIENT_ID: outputValue(
      outputs,
      "seller_user_pool_client_id",
    ),
  };
}

function parseArguments(argv) {
  const options = {
    apply: false,
    initializeSessionKey: false,
    terraformDirectory: "infra/terraform",
    vercelDirectory: "apps/web",
    vercelProject: "web",
    vercelScope: "",
    target: "production",
  };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--apply") options.apply = true;
    else if (argument === "--initialize-session-key") options.initializeSessionKey = true;
    else if (argument === "--terraform-dir") options.terraformDirectory = argv[++index];
    else if (argument === "--vercel-dir") options.vercelDirectory = argv[++index];
    else if (argument === "--project") options.vercelProject = argv[++index];
    else if (argument === "--scope") options.vercelScope = argv[++index];
    else if (argument === "--target") options.target = argv[++index];
    else throw new Error(`Unknown argument: ${argument}`);
  }
  if (options.initializeSessionKey && !options.apply) {
    throw new Error("--initialize-session-key requires --apply.");
  }
  if (!["production", "preview", "development"].includes(options.target)) {
    throw new Error("--target must be production, preview, or development.");
  }
  if (options.apply && !options.vercelScope) {
    throw new Error("--scope is required with --apply to prevent targeting the wrong Vercel team.");
  }
  return options;
}

function terraformOutputs(terraformDirectory) {
  const executable = process.platform === "win32" ? "terraform.exe" : "terraform";
  const result = spawnSync(executable, [`-chdir=${terraformDirectory}`, "output", "-json"], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  if (result.status !== 0) {
    throw new Error(`terraform output failed: ${result.stderr.trim()}`);
  }
  return JSON.parse(result.stdout);
}

function setVercelEnvironment(name, value, target, options, sensitive = false) {
  const executable = process.platform === "win32" ? "npx.cmd" : "npx";
  const argumentsList = [
    "vercel", "env", "add", name, target, "--force",
    "--cwd", options.vercelDirectory,
    "--project", options.vercelProject,
    "--scope", options.vercelScope,
  ];
  argumentsList.push(sensitive ? "--sensitive" : "--no-sensitive", "--yes");
  const result = spawnSync(executable, argumentsList, {
    encoding: "utf8",
    input: `${value}\n`,
    stdio: ["pipe", "inherit", "inherit"],
  });
  if (result.status !== 0) {
    throw new Error(`Vercel rejected environment variable ${name}.`);
  }
}

function run() {
  const options = parseArguments(process.argv.slice(2));
  const outputs = terraformOutputs(options.terraformDirectory);
  const environment = buildVercelEnvironment(outputs);

  if (!options.apply) {
    process.stdout.write(`${JSON.stringify(environment, null, 2)}\n`);
    process.stdout.write("Dry run only. Re-run with --apply after reviewing these non-secret values.\n");
    return;
  }

  for (const [name, value] of Object.entries(environment)) {
    setVercelEnvironment(name, value, options.target, options, false);
  }
  if (options.initializeSessionKey) {
    setVercelEnvironment(
      "AGENTPAY_SESSION_ENCRYPTION_KEY",
      randomBytes(32).toString("base64url"),
      options.target,
      options,
      true,
    );
  }
  process.stdout.write(
    `Configured ${Object.keys(environment).length} non-secret variables${
      options.initializeSessionKey ? " and generated one encrypted session key" : ""
    }.\n`,
  );
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    run();
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
}
