import { describe, it, expect } from "vitest";
import {
  isValidEmail,
  isValidPassword,
  isValidUUID,
  isValidHostname,
  validateVMConfig,
  sanitizeInput,
  isValidCreditAmount,
} from "./validate";

describe("isValidEmail", () => {
  it("accepts valid emails", () => {
    expect(isValidEmail("user@example.com")).toBe(true);
    expect(isValidEmail("first.last@domain.org")).toBe(true);
    expect(isValidEmail("u+tag@sub.domain.co")).toBe(true);
  });

  it("rejects invalid emails", () => {
    expect(isValidEmail("")).toBe(false);
    expect(isValidEmail("noat")).toBe(false);
    expect(isValidEmail("@domain.com")).toBe(false);
    expect(isValidEmail("user@")).toBe(false);
    expect(isValidEmail("user @domain.com")).toBe(false);
    expect(isValidEmail("user@domain")).toBe(false);
  });

  it("trims whitespace", () => {
    expect(isValidEmail("  user@example.com  ")).toBe(true);
  });

  it("handles non-string input", () => {
    expect(isValidEmail(null as unknown as string)).toBe(false);
    expect(isValidEmail(undefined as unknown as string)).toBe(false);
    expect(isValidEmail(42 as unknown as string)).toBe(false);
  });
});

describe("isValidPassword", () => {
  it("accepts valid passwords", () => {
    expect(isValidPassword("Abcdefg1")).toBe(true);
    expect(isValidPassword("MyP@ssw0rd!")).toBe(true);
  });

  it("rejects too short", () => {
    expect(isValidPassword("Ab1")).toBe(false);
    expect(isValidPassword("Abcdef7")).toBe(false);
  });

  it("rejects too long", () => {
    expect(isValidPassword("A1" + "a".repeat(127))).toBe(false);
  });

  it("requires uppercase", () => {
    expect(isValidPassword("abcdefg1")).toBe(false);
  });

  it("requires lowercase", () => {
    expect(isValidPassword("ABCDEFG1")).toBe(false);
  });

  it("requires digit", () => {
    expect(isValidPassword("Abcdefgh")).toBe(false);
  });

  it("handles non-string input", () => {
    expect(isValidPassword("" as string)).toBe(false);
    expect(isValidPassword(null as unknown as string)).toBe(false);
  });
});

describe("isValidUUID", () => {
  it("accepts valid UUIDs", () => {
    expect(isValidUUID("550e8400-e29b-41d4-a716-446655440000")).toBe(true);
    expect(isValidUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")).toBe(true);
  });

  it("rejects invalid UUIDs", () => {
    expect(isValidUUID("")).toBe(false);
    expect(isValidUUID("not-a-uuid")).toBe(false);
    expect(isValidUUID("550e8400-e29b-41d4-a716")).toBe(false);
    expect(isValidUUID("550e8400e29b41d4a716446655440000")).toBe(false);
  });

  it("is case-insensitive", () => {
    expect(isValidUUID("550E8400-E29B-41D4-A716-446655440000")).toBe(true);
  });
});

describe("isValidHostname", () => {
  it("accepts valid hostnames", () => {
    expect(isValidHostname("example.com")).toBe(true);
    expect(isValidHostname("sub.domain.org")).toBe(true);
    expect(isValidHostname("my-host.example.co")).toBe(true);
  });

  it("rejects invalid hostnames", () => {
    expect(isValidHostname("")).toBe(false);
    expect(isValidHostname("-invalid.com")).toBe(false);
    expect(isValidHostname("a".repeat(254) + ".com")).toBe(false);
  });
});

describe("validateVMConfig", () => {
  it("returns no errors for valid config", () => {
    const errors = validateVMConfig({
      cpuCores: 4,
      ramGB: 16,
      storageGB: 100,
    });
    expect(errors).toHaveLength(0);
  });

  it("validates with optional GPU", () => {
    const errors = validateVMConfig({
      cpuCores: 8,
      ramGB: 32,
      storageGB: 200,
      gpuVramGB: 16,
    });
    expect(errors).toHaveLength(0);
  });

  it("catches invalid cpuCores", () => {
    const errors = validateVMConfig({
      cpuCores: 0,
      ramGB: 8,
      storageGB: 50,
    });
    expect(errors).toContain("cpuCores must be a positive integer");
  });

  it("catches excessive cpuCores", () => {
    const errors = validateVMConfig({
      cpuCores: 128,
      ramGB: 8,
      storageGB: 50,
    });
    expect(errors).toContain("cpuCores cannot exceed 64");
  });

  it("catches invalid ramGB", () => {
    const errors = validateVMConfig({
      cpuCores: 4,
      ramGB: -1,
      storageGB: 50,
    });
    expect(errors).toContain("ramGB must be a positive number");
  });

  it("catches excessive ramGB", () => {
    const errors = validateVMConfig({
      cpuCores: 4,
      ramGB: 1024,
      storageGB: 50,
    });
    expect(errors).toContain("ramGB cannot exceed 512");
  });

  it("catches negative storageGB", () => {
    const errors = validateVMConfig({
      cpuCores: 4,
      ramGB: 8,
      storageGB: -10,
    });
    expect(errors).toContain("storageGB cannot be negative");
  });

  it("catches excessive GPU", () => {
    const errors = validateVMConfig({
      cpuCores: 4,
      ramGB: 8,
      storageGB: 50,
      gpuVramGB: 100,
    });
    expect(errors).toContain("gpuVramGB cannot exceed 80");
  });

  it("reports multiple errors", () => {
    const errors = validateVMConfig({
      cpuCores: -1,
      ramGB: -1,
      storageGB: -1,
    });
    expect(errors.length).toBeGreaterThanOrEqual(3);
  });
});

describe("sanitizeInput", () => {
  it("escapes HTML characters", () => {
    expect(sanitizeInput('<script>alert("xss")</script>')).toBe(
      "&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;"
    );
  });

  it("escapes ampersands", () => {
    expect(sanitizeInput("a & b")).toBe("a &amp; b");
  });

  it("escapes single quotes", () => {
    expect(sanitizeInput("it's")).toBe("it&#x27;s");
  });

  it("handles empty string", () => {
    expect(sanitizeInput("")).toBe("");
  });

  it("passes through safe text", () => {
    expect(sanitizeInput("hello world 123")).toBe("hello world 123");
  });

  it("handles non-string input", () => {
    expect(sanitizeInput(null as unknown as string)).toBe("");
  });
});

describe("isValidCreditAmount", () => {
  it("accepts valid amounts", () => {
    expect(isValidCreditAmount(1)).toBe(true);
    expect(isValidCreditAmount(99.99)).toBe(true);
    expect(isValidCreditAmount(10000)).toBe(true);
    expect(isValidCreditAmount(0.01)).toBe(true);
  });

  it("rejects zero", () => {
    expect(isValidCreditAmount(0)).toBe(false);
  });

  it("rejects negative", () => {
    expect(isValidCreditAmount(-5)).toBe(false);
  });

  it("rejects over max", () => {
    expect(isValidCreditAmount(10001)).toBe(false);
  });

  it("rejects too many decimals", () => {
    expect(isValidCreditAmount(1.999)).toBe(false);
    expect(isValidCreditAmount(0.001)).toBe(false);
  });

  it("rejects non-finite", () => {
    expect(isValidCreditAmount(Infinity)).toBe(false);
    expect(isValidCreditAmount(NaN)).toBe(false);
  });
});
