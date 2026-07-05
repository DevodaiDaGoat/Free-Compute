"use client";

interface WindowControlsProps {
  closable: boolean;
  resizable: boolean;
  onMinimize: () => void;
  onMaximize: () => void;
  onClose: () => void;
}

export default function WindowControls({
  closable,
  resizable,
  onMinimize,
  onMaximize,
  onClose,
}: WindowControlsProps) {
  return (
    <div className="flex items-center gap-1.5">
      <button
        type="button"
        aria-label="Minimize"
        onClick={onMinimize}
        onPointerDown={(event) => event.stopPropagation()}
        className="grid h-3.5 w-3.5 place-items-center rounded-full bg-yellow-400 text-[8px] text-black/60 hover:brightness-110"
      >
        –
      </button>
      {resizable && (
        <button
          type="button"
          aria-label="Maximize"
          onClick={onMaximize}
          onPointerDown={(event) => event.stopPropagation()}
          className="grid h-3.5 w-3.5 place-items-center rounded-full bg-green-500 text-[8px] text-black/60 hover:brightness-110"
        >
          ▢
        </button>
      )}
      {closable && (
        <button
          type="button"
          aria-label="Close"
          onClick={onClose}
          onPointerDown={(event) => event.stopPropagation()}
          className="grid h-3.5 w-3.5 place-items-center rounded-full bg-red-500 text-[8px] text-black/60 hover:brightness-110"
        >
          ✕
        </button>
      )}
    </div>
  );
}
