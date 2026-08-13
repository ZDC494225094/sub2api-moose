const insufficientBalanceEvent = "canvas-insufficient-balance";

export function reportInsufficientBalance(error: unknown) {
    const message = error instanceof Error ? error.message : String(error || "");
    if (!/insufficient\s+balance/i.test(message)) return false;
    window.dispatchEvent(new CustomEvent(insufficientBalanceEvent, { detail: message }));
    return true;
}

export function onInsufficientBalance(listener: (message: string) => void) {
    const handler = (event: Event) => listener(event instanceof CustomEvent && typeof event.detail === "string" ? event.detail : "");
    window.addEventListener(insufficientBalanceEvent, handler);
    return () => window.removeEventListener(insufficientBalanceEvent, handler);
}
