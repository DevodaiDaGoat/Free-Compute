import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  timeAgo,
  isExpired,
  addMinutes,
  addHours,
  addDays,
  startOfDay,
  endOfDay,
  isSameDay,
  diffInMinutes,
  diffInHours,
  formatISODate,
} from "./date";

describe("timeAgo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-06-01T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows seconds ago", () => {
    const d = new Date("2025-06-01T11:59:30Z");
    expect(timeAgo(d)).toBe("30s ago");
  });

  it("shows minutes ago", () => {
    const d = new Date("2025-06-01T11:55:00Z");
    expect(timeAgo(d)).toBe("5m ago");
  });

  it("shows hours ago", () => {
    const d = new Date("2025-06-01T09:00:00Z");
    expect(timeAgo(d)).toBe("3h ago");
  });

  it("shows days ago", () => {
    const d = new Date("2025-05-29T12:00:00Z");
    expect(timeAgo(d)).toBe("3d ago");
  });

  it("shows months ago", () => {
    const d = new Date("2025-03-01T12:00:00Z");
    expect(timeAgo(d)).toBe("3mo ago");
  });

  it("shows years ago", () => {
    const d = new Date("2023-06-01T12:00:00Z");
    expect(timeAgo(d)).toBe("2y ago");
  });

  it("handles just now", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    expect(timeAgo(d)).toBe("just now");
  });

  it("handles future dates", () => {
    const d = new Date("2025-06-02T12:00:00Z");
    expect(timeAgo(d)).toBe("just now");
  });

  it("accepts string input", () => {
    expect(timeAgo("2025-06-01T11:59:00Z")).toBe("1m ago");
  });

  it("accepts timestamp input", () => {
    const ts = new Date("2025-06-01T11:00:00Z").getTime();
    expect(timeAgo(ts)).toBe("1h ago");
  });

  it("handles invalid date", () => {
    expect(timeAgo("not-a-date")).toBe("unknown");
  });
});

describe("isExpired", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-06-01T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns true for past dates", () => {
    expect(isExpired(new Date("2025-05-01T00:00:00Z"))).toBe(true);
  });

  it("returns false for future dates", () => {
    expect(isExpired(new Date("2025-07-01T00:00:00Z"))).toBe(false);
  });

  it("accepts string input", () => {
    expect(isExpired("2025-05-01T00:00:00Z")).toBe(true);
  });

  it("accepts timestamp input", () => {
    const future = Date.now() + 60000;
    expect(isExpired(future)).toBe(false);
  });

  it("returns true for invalid date", () => {
    expect(isExpired("invalid")).toBe(true);
  });
});

describe("addMinutes", () => {
  it("adds positive minutes", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    const result = addMinutes(d, 30);
    expect(result.toISOString()).toBe("2025-06-01T12:30:00.000Z");
  });

  it("adds negative minutes", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    const result = addMinutes(d, -15);
    expect(result.toISOString()).toBe("2025-06-01T11:45:00.000Z");
  });

  it("does not mutate original", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    addMinutes(d, 30);
    expect(d.toISOString()).toBe("2025-06-01T12:00:00.000Z");
  });
});

describe("addHours", () => {
  it("adds hours", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    expect(addHours(d, 3).toISOString()).toBe("2025-06-01T15:00:00.000Z");
  });

  it("crosses day boundary", () => {
    const d = new Date("2025-06-01T23:00:00Z");
    expect(addHours(d, 2).toISOString()).toBe("2025-06-02T01:00:00.000Z");
  });
});

describe("addDays", () => {
  it("adds days", () => {
    const d = new Date("2025-06-01T12:00:00Z");
    expect(addDays(d, 5).toISOString()).toBe("2025-06-06T12:00:00.000Z");
  });

  it("crosses month boundary", () => {
    const d = new Date("2025-06-28T12:00:00Z");
    expect(addDays(d, 5).toISOString()).toBe("2025-07-03T12:00:00.000Z");
  });
});

describe("startOfDay", () => {
  it("sets time to midnight", () => {
    const d = new Date("2025-06-01T15:30:45.123Z");
    const s = startOfDay(d);
    expect(s.getHours()).toBe(0);
    expect(s.getMinutes()).toBe(0);
    expect(s.getSeconds()).toBe(0);
    expect(s.getMilliseconds()).toBe(0);
  });

  it("does not mutate original", () => {
    const d = new Date("2025-06-01T15:30:00Z");
    startOfDay(d);
    expect(d.getUTCHours()).toBe(15);
  });
});

describe("endOfDay", () => {
  it("sets time to end of day", () => {
    const d = new Date("2025-06-01T10:00:00Z");
    const e = endOfDay(d);
    expect(e.getHours()).toBe(23);
    expect(e.getMinutes()).toBe(59);
    expect(e.getSeconds()).toBe(59);
    expect(e.getMilliseconds()).toBe(999);
  });
});

describe("isSameDay", () => {
  it("returns true for same day", () => {
    const a = new Date("2025-06-01T08:00:00Z");
    const b = new Date("2025-06-01T20:00:00Z");
    expect(isSameDay(a, b)).toBe(true);
  });

  it("returns false for different days", () => {
    const a = new Date("2025-06-01T08:00:00Z");
    const b = new Date("2025-06-02T08:00:00Z");
    expect(isSameDay(a, b)).toBe(false);
  });
});

describe("diffInMinutes", () => {
  it("calculates difference", () => {
    const a = new Date("2025-06-01T12:00:00Z");
    const b = new Date("2025-06-01T12:45:00Z");
    expect(diffInMinutes(a, b)).toBe(45);
  });

  it("is absolute", () => {
    const a = new Date("2025-06-01T12:45:00Z");
    const b = new Date("2025-06-01T12:00:00Z");
    expect(diffInMinutes(a, b)).toBe(45);
  });
});

describe("diffInHours", () => {
  it("calculates difference", () => {
    const a = new Date("2025-06-01T12:00:00Z");
    const b = new Date("2025-06-01T15:00:00Z");
    expect(diffInHours(a, b)).toBe(3);
  });
});

describe("formatISODate", () => {
  it("returns date portion only", () => {
    const d = new Date("2025-06-01T15:30:00Z");
    expect(formatISODate(d)).toBe("2025-06-01");
  });
});
