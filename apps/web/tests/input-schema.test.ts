import { describe, expect, it } from "vitest";
import {
  buildSchemaDocument,
  validatePublishedInput,
  type PublishedInputSchema,
} from "@/features/commerce/input-schema";

const schema: PublishedInputSchema = {
  type: "object",
  additionalProperties: false,
  required: ["productName", "audience"],
  properties: {
    productName: { type: "string", minLength: 2, maxLength: 20 },
    audience: { type: "string", enum: ["Founders", "Developers"] },
    copies: { type: "integer", minimum: 1, maximum: 5 },
    details: {
      type: "object",
      additionalProperties: false,
      required: ["notes"],
      properties: { notes: { type: "string", maxLength: 5 } },
    },
  },
};

describe("published checkout input schema", () => {
  it("enforces required, unknown, type, enum, nested, and range rules", () => {
    expect(
      validatePublishedInput(schema, { productName: "AgentPay" }),
    ).toContainEqual({
      field: "audience",
      message: "Audience is required.",
    });
    expect(
      validatePublishedInput(schema, {
        productName: "AgentPay",
        audience: "Founders",
        secret: true,
      }),
    ).toContainEqual({
      field: "secret",
      message: "Secret is not accepted by this product.",
    });
    expect(
      validatePublishedInput(schema, {
        productName: 42,
        audience: "Everyone",
        copies: 6,
        details: { notes: "too long" },
      }).map((issue) => issue.field),
    ).toEqual(["audience", "copies", "details.notes", "productName"]);
  });

  it("builds typed nested JSON from the generated form drafts", () => {
    expect(
      buildSchemaDocument(schema, {
        productName: "AgentPay",
        audience: "Founders",
        copies: "2",
        "details.notes": "brief",
      }),
    ).toEqual({
      value: {
        audience: "Founders",
        copies: 2,
        details: { notes: "brief" },
        productName: "AgentPay",
      },
      issues: [],
    });
  });
});
