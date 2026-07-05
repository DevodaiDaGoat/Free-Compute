import { describe, it, expect } from "vitest";
import {
  formatBytes,
  formatDuration,
  formatCredits,
  formatCPU,
  formatRAM,
  formatPercent,
  truncate,
  slugify,
} from "./format";

describe("formatBytes", () => {
  it("formats zero bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("formats bytes", () => {
    expect(formatBytes(500)).toBe("500 B");
  });

  it("formats kilobytes", () => {
    expect(formatBytes(1024)).toBe("1.00 KB");
    expect(formatBytes(1536)).toBe("1.50 KB");
  });

  it("formats megabytes", () => {
    expect(formatBytes(1048576)).toBe("1.00 MB");
  });

  it("formats gigabytes", () => {
    expect(formatBytes(1073741824)).toBe("1.00 GB");
  });

  it("formats terabytes", () => {
    expect(formatBytes(1099511627776)).toBe("1.00 TB");
  });

  it("handles negative input", () => {
    expect(formatBytes(-1)).toBe("0 B");
  });

  it("handles non-finite input", () => {
    expect(formatBytes(NaN)).toBe("0 B");
    expect(formatBytes(Infinity)).toBe("0 B");
  });
});

describe("formatDuration", () => {
  it("formats seconds", () => {
    expect(formatDuration(5000)).toBe("5s");
    expect(formatDuration(0)).toBe("0s");
  });

  it("formats minutes and seconds", () => {
    expect(formatDuration(90000)).toBe("1m 30s");
  });

  it("formats hours and minutes", () => {
    expect(formatDuration(3720000)).toBe("1h 2m");
  });

  it("formats days and hours", () => {
    expect(formatDuration(90000000)).toBe("1d 1h");
  });

  it("handles negative input", () => {
    expect(formatDuration(-1000)).toBe("0s");
  });

  it("handles non-finite input", () => {
    expect(formatDuration(NaN)).toBe("0s");
  });
});

describe("formatCredits", () => {
  it("formats to two decimals", () => {
    expect(formatCredits(10)).toBe("10.00");
    expect(formatCredits(99.9)).toBe("99.90");
    expect(formatCredits(0.1)).toBe("0.10");
  });

  it("handles zero", () => {
    expect(formatCredits(0)).toBe("0.00");
  });

  it("handles non-finite", () => {
    expect(formatCredits(NaN)).toBe("0.00");
  });
});

describe("formatCPU", () => {
  it("singular for 1 core", () => {
    expect(formatCPU(1)).toBe("1 core");
  });

  it("plural for multiple", () => {
    expect(formatCPU(4)).toBe("4 cores");
    expect(formatCPU(16)).toBe("16 cores");
  });

  it("handles zero", () => {
    expect(formatCPU(0)).toBe("0 cores");
  });

  it("handles negative", () => {
    expect(formatCPU(-1)).toBe("0 cores");
  });
});

describe("formatRAM", () => {
  it("formats GB values", () => {
    expect(formatRAM(16)).toBe("16 GB");
    expect(formatRAM(128)).toBe("128 GB");
  });

  it("formats sub-GB as MB", () => {
    expect(formatRAM(0.5)).toBe("512 MB");
  });

  it("handles zero", () => {
    expect(formatRAM(0)).toBe("0 GB");
  });

  it("handles negative", () => {
    expect(formatRAM(-1)).toBe("0 GB");
  });
});

describe("formatPercent", () => {
  it("formats with default decimals", () => {
    expect(formatPercent(50)).toBe("50.0%");
    expect(formatPercent(99.99)).toBe("100.0%");
  });

  it("formats with custom decimals", () => {
    expect(formatPercent(33.333, 2)).toBe("33.33%");
    expect(formatPercent(100, 0)).toBe("100%");
  });

  it("handles non-finite", () => {
    expect(formatPercent(NaN)).toBe("0%");
  });
});

describe("truncate", () => {
  it("returns short strings unchanged", () => {
    expect(truncate("hello", 10)).toBe("hello");
  });

  it("truncates with ellipsis", () => {
    expect(truncate("hello world", 8)).toBe("hello...");
  });

  it("handles maxLength <= 3", () => {
    expect(truncate("hello", 3)).toBe("hel");
    expect(truncate("hello", 1)).toBe("h");
  });

  it("handles empty string", () => {
    expect(truncate("", 10)).toBe("");
  });

  it("handles non-string input", () => {
    expect(truncate(null as unknown as string, 10)).toBe("");
  });

  it("handles exact length", () => {
    expect(truncate("hello", 5)).toBe("hello");
  });
});

describe("slugify", () => {
  it("converts to lowercase slug", () => {
    expect(slugify("Hello World")).toBe("hello-world");
  });

  it("removes special characters", () => {
    expect(slugify("Hello, World!")).toBe("hello-world");
  });

  it("collapses multiple spaces/hyphens", () => {
    expect(slugify("hello   world")).toBe("hello-world");
    expect(slugify("hello---world")).toBe("hello-world");
  });

  it("trims leading/trailing hyphens", () => {
    expect(slugify("  hello world  ")).toBe("hello-world");
  });

  it("handles empty string", () => {
    expect(slugify("")).toBe("");
  });

  it("handles non-string", () => {
    expect(slugify(null as unknown as string)).toBe("");
  });

  it("converts underscores to hyphens", () => {
    expect(slugify("hello_world")).toBe("hello-world");
  });
});
