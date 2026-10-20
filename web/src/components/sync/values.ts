export const syncKeys = {
  all: ["sync"],
  capabilities: ["sync", "capabilities"],
  rules: ["sync", "rules"],
  detail: (id: number) => ["sync", "detail", id],
};
export const statusLabels: Record<string, string> = {
  synced: "Synced",
  no_change: "No change needed",
  needs_mapping: "Needs mapping",
  needs_sign_in: "Needs sign-in",
  needs_review: "Needs review",
  failed: "Failed",
  queued: "Queued",
  processing: "Processing",
  received: "Not recorded",
  cancelled: "Cancelled",
};
export function displayValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return "Not set";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (typeof value === "object")
    return Object.values(value).map(displayValue).join(" · ");
  return String(value).replaceAll("_", " ");
}
