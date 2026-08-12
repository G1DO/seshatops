/** Explicit connection presentation states (M1-INV-11). */
export type ConnectionState =
  | "loading"
  | "live"
  | "stale"
  | "disconnected"
  | "error";

export function connectionLabel(state: ConnectionState): string {
  switch (state) {
    case "loading":
      return "Loading projection…";
    case "live":
      return "Live — connected to projection stream";
    case "stale":
      return "Stale — stream interrupted; refetching authoritative snapshot";
    case "disconnected":
      return "Disconnected — projection may be out of date";
    case "error":
      return "Error — could not load authoritative projection";
  }
}

/** Stale/disconnected must never be presented as unquestionably current. */
export function isAuthoritativeLive(state: ConnectionState): boolean {
  return state === "live";
}
