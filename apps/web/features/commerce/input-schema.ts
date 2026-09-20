export type PublishedInputSchema = {
  type?: string;
  properties?: Record<string, PublishedInputSchema>;
  required?: string[];
  additionalProperties?: boolean;
  description?: string;
  format?: string;
  enum?: unknown[];
  const?: unknown;
  items?: PublishedInputSchema;
  minLength?: number;
  maxLength?: number;
  minimum?: number;
  maximum?: number;
  minItems?: number;
  maxItems?: number;
  pattern?: string;
  oneOf?: PublishedInputSchema[];
  anyOf?: PublishedInputSchema[];
  allOf?: PublishedInputSchema[];
};

export type InputDrafts = Record<string, string | boolean>;

export type InputValidationIssue = {
  field: string;
  message: string;
};

export function asPublishedInputSchema(
  schema: Record<string, unknown>,
): PublishedInputSchema {
  return schema as PublishedInputSchema;
}

export function schemaPropertyEntries(schema: PublishedInputSchema) {
  return Object.entries(schema.properties ?? {}).sort(([left], [right]) =>
    left.localeCompare(right),
  );
}

export function schemaFieldLabel(name: string): string {
  const words = name
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim();
  return words
    ? words[0].toUpperCase() + words.slice(1).toLowerCase()
    : "Field";
}

export function buildSchemaDocument(
  schema: PublishedInputSchema,
  drafts: InputDrafts,
): { value?: Record<string, unknown>; issues: InputValidationIssue[] } {
  const value = buildObjectValue(schema, drafts, "");
  if (value.issues.length > 0) return value;
  const issues = validatePublishedInput(schema, value.value ?? {});
  return issues.length > 0
    ? { issues }
    : { value: value.value ?? {}, issues: [] };
}

export function validatePublishedInput(
  schema: PublishedInputSchema,
  value: unknown,
  path = "",
): InputValidationIssue[] {
  const field = path || "Request body";
  const issues: InputValidationIssue[] = [];

  if (Object.hasOwn(schema, "const") && !jsonEqual(value, schema.const)) {
    return [{ field, message: `${field} must match the published value.` }];
  }
  if (schema.enum && !schema.enum.some((choice) => jsonEqual(choice, value))) {
    return [
      { field, message: `${field} must be one of the published choices.` },
    ];
  }
  if (schema.type && !matchesType(schema.type, value)) {
    return [{ field, message: `${field} must be ${typeLabel(schema.type)}.` }];
  }

  if (isRecord(value)) {
    const properties = schema.properties ?? {};
    for (const requiredName of schema.required ?? []) {
      if (!Object.hasOwn(value, requiredName)) {
        issues.push({
          field: joinPath(path, requiredName),
          message: `${schemaFieldLabel(requiredName)} is required.`,
        });
      }
    }
    for (const name of Object.keys(value).sort()) {
      if (!Object.hasOwn(properties, name)) {
        issues.push({
          field: joinPath(path, name),
          message: `${schemaFieldLabel(name)} is not accepted by this product.`,
        });
      }
    }
    for (const [name, propertySchema] of schemaPropertyEntries(schema)) {
      if (Object.hasOwn(value, name)) {
        issues.push(
          ...validatePublishedInput(
            propertySchema,
            value[name],
            joinPath(path, name),
          ),
        );
      }
    }
  } else if (Array.isArray(value)) {
    if (schema.minItems !== undefined && value.length < schema.minItems) {
      issues.push({
        field,
        message: `${field} must contain at least ${schema.minItems} items.`,
      });
    }
    if (schema.maxItems !== undefined && value.length > schema.maxItems) {
      issues.push({
        field,
        message: `${field} must contain at most ${schema.maxItems} items.`,
      });
    }
    if (schema.items) {
      value.forEach((item, index) => {
        issues.push(
          ...validatePublishedInput(schema.items!, item, `${path}[${index}]`),
        );
      });
    }
  } else if (typeof value === "string") {
    const label = schemaFieldLabel(path.split(".").at(-1) ?? path);
    if (schema.minLength !== undefined && value.length < schema.minLength) {
      issues.push({
        field,
        message: `${label} must be at least ${schema.minLength} characters.`,
      });
    }
    if (schema.maxLength !== undefined && value.length > schema.maxLength) {
      issues.push({
        field,
        message: `${label} must be at most ${schema.maxLength} characters.`,
      });
    }
    if (schema.pattern) {
      try {
        if (!new RegExp(schema.pattern).test(value)) {
          issues.push({ field, message: `${label} has an invalid format.` });
        }
      } catch {
        issues.push({ field, message: `${label} has an invalid format.` });
      }
    }
    if (schema.format && !matchesStringFormat(schema.format, value)) {
      issues.push({
        field,
        message: `${label} must be a valid ${schema.format}.`,
      });
    }
  } else if (typeof value === "number") {
    const label = schemaFieldLabel(path.split(".").at(-1) ?? path);
    if (schema.minimum !== undefined && value < schema.minimum) {
      issues.push({
        field,
        message: `${label} must be at least ${schema.minimum}.`,
      });
    }
    if (schema.maximum !== undefined && value > schema.maximum) {
      issues.push({
        field,
        message: `${label} must be at most ${schema.maximum}.`,
      });
    }
  }

  issues.push(...validateCompositions(schema, value, path));
  return issues;
}

function buildObjectValue(
  schema: PublishedInputSchema,
  drafts: InputDrafts,
  path: string,
): { value?: Record<string, unknown>; issues: InputValidationIssue[] } {
  const value: Record<string, unknown> = {};
  const issues: InputValidationIssue[] = [];
  for (const [name, propertySchema] of schemaPropertyEntries(schema)) {
    const propertyPath = joinPath(path, name);
    if (Object.hasOwn(propertySchema, "const")) {
      value[name] = propertySchema.const;
      continue;
    }
    if (propertySchema.type === "object") {
      const nested = buildObjectValue(propertySchema, drafts, propertyPath);
      issues.push(...nested.issues);
      if (nested.value && Object.keys(nested.value).length > 0)
        value[name] = nested.value;
      continue;
    }
    const draft = drafts[propertyPath];
    if (propertySchema.type === "boolean") {
      if (draft === true || schema.required?.includes(name)) {
        value[name] = draft === true;
      }
      continue;
    }
    if (typeof draft !== "string" || draft.trim() === "") continue;
    if (propertySchema.type === "number" || propertySchema.type === "integer") {
      const number = Number(draft);
      if (!Number.isFinite(number)) {
        issues.push({
          field: propertyPath,
          message: `${schemaFieldLabel(name)} must be a number.`,
        });
      } else {
        value[name] = number;
      }
      continue;
    }
    if (propertySchema.type === "array") {
      try {
        const parsed = JSON.parse(draft) as unknown;
        if (!Array.isArray(parsed)) throw new Error("not an array");
        value[name] = parsed;
      } catch {
        issues.push({
          field: propertyPath,
          message: `${schemaFieldLabel(name)} must be a valid JSON array.`,
        });
      }
      continue;
    }
    value[name] = draft;
  }
  return { value, issues };
}

function validateCompositions(
  schema: PublishedInputSchema,
  value: unknown,
  path: string,
): InputValidationIssue[] {
  const field = path || "Request body";
  const issues: InputValidationIssue[] = [];
  for (const alternative of schema.allOf ?? []) {
    issues.push(...validatePublishedInput(alternative, value, path));
  }
  if (
    schema.anyOf &&
    !schema.anyOf.some(
      (alternative) =>
        validatePublishedInput(alternative, value, path).length === 0,
    )
  ) {
    issues.push({
      field,
      message: `${field} does not match an accepted option.`,
    });
  }
  if (schema.oneOf) {
    const matches = schema.oneOf.filter(
      (alternative) =>
        validatePublishedInput(alternative, value, path).length === 0,
    ).length;
    if (matches !== 1)
      issues.push({
        field,
        message: `${field} must match exactly one accepted option.`,
      });
  }
  return issues;
}

function matchesType(type: string, value: unknown): boolean {
  switch (type) {
    case "object":
      return isRecord(value);
    case "array":
      return Array.isArray(value);
    case "string":
      return typeof value === "string";
    case "number":
      return typeof value === "number" && Number.isFinite(value);
    case "integer":
      return typeof value === "number" && Number.isInteger(value);
    case "boolean":
      return typeof value === "boolean";
    case "null":
      return value === null;
    default:
      return false;
  }
}

function typeLabel(type: string): string {
  return type === "object" || type === "array" ? `a JSON ${type}` : `a ${type}`;
}

function joinPath(path: string, name: string): string {
  return path ? `${path}.${name}` : name;
}

function jsonEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

function matchesStringFormat(format: string, value: string): boolean {
  switch (format) {
    case "email":
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
    case "uri": {
      try {
        return Boolean(new URL(value).protocol);
      } catch {
        return false;
      }
    }
    case "date-time":
      return !Number.isNaN(Date.parse(value)) && value.includes("T");
    default:
      return true;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
