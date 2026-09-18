import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";

const repositoryRoot = process.cwd();
const dockerImages = {
  dotnet:
    "mcr.microsoft.com/dotnet/sdk:8.0@sha256:5ef85cc12cb25be6ec319a7392d1e9efd53c3bc8abb971c53d8058a473f09053",
  php: "php:8.3-cli@sha256:4bc53b7a9d8acad2b8109e4fedfc4b9cd9a07a97fc153d1eb2bdfa8c64d903a5",
  ruby: "ruby:3.3@sha256:d1041e8de1eaf1e2f7004ed0298fd37baa0b090979c80cf84c8a6696c0766812",
};

// run executes one required command and stops on any failure.
function run(command, argumentsList, options = {}) {
  const result = spawnSync(command, argumentsList, {
    cwd: options.cwd ?? repositoryRoot,
    env: process.env,
    stdio: "inherit",
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

// commandExists reports whether a local runtime can start successfully.
function commandExists(command, versionArguments = ["--version"]) {
  const result = spawnSync(command, versionArguments, {
    cwd: repositoryRoot,
    env: process.env,
    stdio: "ignore",
  });
  return !result.error && result.status === 0;
}

// runInDocker executes one package check in a pinned official runtime image.
function runInDocker(image, workdir, commandArguments) {
  run("docker", [
    "run",
    "--rm",
    "-v",
    `${repositoryRoot}:/workspace`,
    "-w",
    workdir,
    image,
    ...commandArguments,
  ]);
}

// runDotNetTests verifies the ASP.NET Core package on .NET 8.
function runDotNetTests() {
  const project = "verification/dotnet/AgentPay.Verify.Tests/AgentPay.Verify.Tests.csproj";
  if (commandExists("dotnet")) {
    run("dotnet", ["run", "--project", project, "--artifacts-path", path.join(tmpdir(), "agentpay-dotnet")]);
    return;
  }
  runInDocker(
    dockerImages.dotnet,
    "/workspace",
    ["dotnet", "run", "--project", project, "--artifacts-path", "/tmp/agentpay-dotnet"],
  );
}

// runJavaTests compiles the Spring Boot verification primitive and fixtures.
function runJavaTests() {
  if (!commandExists("javac") || !commandExists("java", ["-version"])) {
    throw new Error("Java 17 or newer is required to run extended verification tests.");
  }
  const outputDirectory = mkdtempSync(path.join(tmpdir(), "agentpay-java-"));
  try {
    run("javac", [
      "--release",
      "17",
      "-d",
      outputDirectory,
      "verification/java/src/main/java/com/agentpay/verify/AgentPayVerifier.java",
      "verification/java/src/main/java/com/agentpay/verify/AgentPayVerificationFilter.java",
      "verification/java/src/test/java/com/agentpay/verify/AgentPayVerifierTest.java",
    ]);
    run("java", ["-cp", outputDirectory, "com.agentpay.verify.AgentPayVerifierTest"]);
  } finally {
    rmSync(outputDirectory, { force: true, recursive: true });
  }
}

// runRubyTests verifies the Rails Rack middleware package.
function runRubyTests() {
  if (commandExists("ruby")) {
    run("ruby", ["-Ilib", "test/verify_test.rb"], {
      cwd: path.join(repositoryRoot, "verification", "ruby"),
    });
    return;
  }
  runInDocker(
    dockerImages.ruby,
    "/workspace/verification/ruby",
    ["ruby", "-Ilib", "test/verify_test.rb"],
  );
}

// runPHPTests verifies the Laravel middleware package.
function runPHPTests() {
  if (commandExists("php")) {
    run("php", ["tests/verify_test.php"], {
      cwd: path.join(repositoryRoot, "verification", "php"),
    });
    return;
  }
  runInDocker(
    dockerImages.php,
    "/workspace/verification/php",
    ["php", "tests/verify_test.php"],
  );
}

runDotNetTests();
runJavaTests();
runRubyTests();
runPHPTests();
