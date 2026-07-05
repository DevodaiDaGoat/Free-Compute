"use client";

import { useState } from "react";

import type { AppWindowProps } from "@/lib/types";
import { cn } from "@/lib/utils";

type Operator = "+" | "-" | "*" | "/";

function compute(a: number, b: number, operator: Operator): number {
  switch (operator) {
    case "+":
      return a + b;
    case "-":
      return a - b;
    case "*":
      return a * b;
    case "/":
      return b === 0 ? NaN : a / b;
  }
}

export default function Calculator(_props: AppWindowProps) {
  void _props;
  const [display, setDisplay] = useState("0");
  const [accumulator, setAccumulator] = useState<number | null>(null);
  const [operator, setOperator] = useState<Operator | null>(null);
  const [waitingForOperand, setWaitingForOperand] = useState(false);

  const inputDigit = (digit: string) => {
    if (waitingForOperand) {
      setDisplay(digit);
      setWaitingForOperand(false);
    } else {
      setDisplay((prev) => (prev === "0" ? digit : prev + digit));
    }
  };

  const inputDecimal = () => {
    if (waitingForOperand) {
      setDisplay("0.");
      setWaitingForOperand(false);
      return;
    }
    if (!display.includes(".")) {
      setDisplay((prev) => prev + ".");
    }
  };

  const clearAll = () => {
    setDisplay("0");
    setAccumulator(null);
    setOperator(null);
    setWaitingForOperand(false);
  };

  const performOperation = (nextOperator: Operator) => {
    const inputValue = parseFloat(display);
    if (accumulator === null) {
      setAccumulator(inputValue);
    } else if (operator) {
      const result = compute(accumulator, inputValue, operator);
      setAccumulator(result);
      setDisplay(String(result));
    }
    setWaitingForOperand(true);
    setOperator(nextOperator);
  };

  const equals = () => {
    if (operator === null || accumulator === null) return;
    const inputValue = parseFloat(display);
    const result = compute(accumulator, inputValue, operator);
    setDisplay(Number.isFinite(result) ? String(result) : "Error");
    setAccumulator(null);
    setOperator(null);
    setWaitingForOperand(true);
  };

  const buttons: Array<{
    label: string;
    onClick: () => void;
    variant?: "accent" | "muted";
    span?: boolean;
  }> = [
    { label: "AC", onClick: clearAll, variant: "muted" },
    { label: "±", onClick: () => setDisplay((p) => String(parseFloat(p) * -1)), variant: "muted" },
    { label: "%", onClick: () => setDisplay((p) => String(parseFloat(p) / 100)), variant: "muted" },
    { label: "÷", onClick: () => performOperation("/"), variant: "accent" },
    { label: "7", onClick: () => inputDigit("7") },
    { label: "8", onClick: () => inputDigit("8") },
    { label: "9", onClick: () => inputDigit("9") },
    { label: "×", onClick: () => performOperation("*"), variant: "accent" },
    { label: "4", onClick: () => inputDigit("4") },
    { label: "5", onClick: () => inputDigit("5") },
    { label: "6", onClick: () => inputDigit("6") },
    { label: "−", onClick: () => performOperation("-"), variant: "accent" },
    { label: "1", onClick: () => inputDigit("1") },
    { label: "2", onClick: () => inputDigit("2") },
    { label: "3", onClick: () => inputDigit("3") },
    { label: "+", onClick: () => performOperation("+"), variant: "accent" },
    { label: "0", onClick: () => inputDigit("0"), span: true },
    { label: ".", onClick: inputDecimal },
    { label: "=", onClick: equals, variant: "accent" },
  ];

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <div className="flex min-h-16 items-end justify-end overflow-hidden rounded-lg bg-black/40 px-4 py-3 font-mono text-3xl tabular-nums text-white">
        {display}
      </div>
      <div className="grid flex-1 grid-cols-4 gap-2">
        {buttons.map((btn, index) => (
          <button
            key={`${btn.label}-${index}`}
            type="button"
            onClick={btn.onClick}
            className={cn(
              "rounded-lg text-lg font-medium transition-colors",
              btn.span && "col-span-2",
              btn.variant === "accent" &&
                "bg-[var(--accent)] text-black hover:opacity-90",
              btn.variant === "muted" &&
                "bg-white/20 text-white hover:bg-white/30",
              !btn.variant && "bg-white/10 text-white hover:bg-white/20",
            )}
          >
            {btn.label}
          </button>
        ))}
      </div>
    </div>
  );
}
