#!/usr/bin/env node
import * as cdk from "aws-cdk-lib";

const app = new cdk.App();
const environment = app.node.tryGetContext("environment") ?? "dev";

const foundation = new cdk.Stack(app, `AgentPayFoundation-${environment}`, {
  description: "AgentPay foundation placeholder; resources are added in AWS-002 through AWS-004.",
});

cdk.Tags.of(foundation).add("Project", "AgentPay");
cdk.Tags.of(foundation).add("Environment", environment);
cdk.Tags.of(foundation).add("ManagedBy", "CDK");

app.synth();
