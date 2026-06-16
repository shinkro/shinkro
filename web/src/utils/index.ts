import {formatDistanceToNowStrict, formatISO9075} from "date-fns";

// sleep for x ms
export function sleep(ms: number) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

// get baseUrl sent from server rendered index template
export function baseUrl() {
    let baseUrl = "/";
    if (window.APP.baseUrl) {
        if (window.APP.baseUrl === "{{.BaseUrl}}") {
            baseUrl = "/";
        } else {
            baseUrl = window.APP.baseUrl.endsWith("/")
                ? window.APP.baseUrl
                : `${window.APP.baseUrl}/`;
        }
    }
    return baseUrl;
}

// get routerBasePath sent from server rendered index template
// routerBasePath is used for RouterProvider and does not need work with trailing slash
export function routerBasePath() {
    let baseUrl = "";
    if (window.APP.baseUrl) {
        if (window.APP.baseUrl === "{{.BaseUrl}}") {
            baseUrl = "";
        } else {
            baseUrl = window.APP.baseUrl;
        }
    }
    return baseUrl;
}

// get sseBaseUrl for SSE
export function sseBaseUrl() {
    if (import.meta.env.DEV) return "http://localhost:7012/";

    return `${window.location.origin}${baseUrl()}`;
}

export function classNames(...classes: string[]) {
    return classes.filter(Boolean).join(" ");
}

// Normalize pathname by removing baseUrl prefix
export function normalizePathname(pathname: string, baseUrl: string): string {
    // If baseUrl is just "/", no normalization needed
    if (baseUrl === "/") {
        return pathname;
    }
    
    // Remove trailing slash from baseUrl for comparison
    const basePath = baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
    
    // If pathname starts with basePath, remove it and ensure leading slash
    if (pathname.startsWith(basePath)) {
        const remaining = pathname.slice(basePath.length);
        return remaining.startsWith("/") ? remaining : `/${remaining}`;
    }
    
    // If no match, return original pathname
    return pathname;
}

// column widths for inputs etc
export type COL_WIDTHS = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12;

// simplify date
export function simplifyDate(date?: string) {
    if (typeof date === "string" && date !== "0001-01-01T00:00:00Z") {
        return formatISO9075(new Date(date));
    }
    return "n/a";
}

// if empty date show as n/a
export function IsEmptyDate(date?: string) {
    if (typeof date === "string" && date !== "0001-01-01T00:00:00Z") {
        return formatDistanceToNowStrict(new Date(date), {addSuffix: true});
    }
    return "n/a";
}

export function slugify(str: string) {
    return str
        .normalize("NFKD")
        .toLowerCase()
        .replace(/[^\w\s-]/g, "")
        .trim()
        .replace(/[-\s]+/g, "-");
}

// WARNING: This is not a drop in replacement solution and
// it might not work for some edge cases. Test your code!
// export const get = <T>(
//   obj: T,
//   path: string | Array<any>,
//   defValue?: string
// ) => {
//   // If path is not defined or it has false value
//   if (!path) return undefined;
//   // Check if path is string or array. Regex : ensure that we do not have '.' and brackets.
//   // Regex explained: https://regexr.com/58j0k
//   const pathArray = Array.isArray(path) ? path : path.match(/([^[[.\]])+/g);
//   // Find value
//   const result =
//     pathArray &&
//     pathArray.reduce((prevObj, key) => prevObj && prevObj[key], obj);
//   // If found value is undefined return default value; otherwise return the value
//   return result === undefined ? defValue : result;
// };

const UNITS = [
    "byte",
    "kilobyte",
    "megabyte",
    "gigabyte",
    "terabyte",
    "petabyte",
];
const BYTES_PER_KB = 1000;

/**
 * Format bytes as human-readable text.
 *
 * @param sizeBytes Number of bytes.
 *
 * @return Formatted string.
 */
export function humanFileSize(sizeBytes: number | bigint): string {
    let size = Math.abs(Number(sizeBytes));

    let u = 0;
    while (size >= BYTES_PER_KB && u < UNITS.length - 1) {
        size /= BYTES_PER_KB;
        ++u;
    }

    return new Intl.NumberFormat([], {
        style: "unit",
        unit: UNITS[u],
        unitDisplay: "short",
        maximumFractionDigits: 1,
    }).format(size);
}

// Helpers for UI formatting
export function safeDate(date?: string) {
    return date && date.trim().length > 0 ? date : "Not set";
}

export function statusColor(status: string) {
    if (status === "watching") return "green";
    if (status === "completed") return "blue";
    if (status === "on_hold") return "yellow";
    if (status === "dropped") return "red";
    if (status === "plan_to_watch") return "plex";
    return "gray";
}

// Replace underscores with spaces; optionally capitalize each word
export function formatStatusLabel(status?: string, capitalizeWords = true) {
    if (!status) return "";
    const words = status.split("_").filter(Boolean);
    if (!capitalizeWords) return words.join(" ");
    return words.map((w) => (w.length ? w[0].toUpperCase() + w.slice(1) : w)).join(" ");
}

export async function CopyTextToClipboard(text: string) {
    if ("clipboard" in navigator) {
        // Safari requires clipboard operations to be directly triggered by a user interaction.
        // Using setTimeout with a delay of 0 ensures the clipboard operation is deferred until
        // after the current call stack has cleared, effectively placing it outside of the
        // immediate execution context of the user interaction event. This workaround allows
        // the clipboard operation to bypass Safari's security restrictions.
        setTimeout(async () => {
            try {
                await navigator.clipboard.writeText(text);
                console.log("Text copied to clipboard successfully.");
            } catch (err) {
                console.error("Copy to clipboard unsuccessful: ", err);
            }
        }, 0);
    } else {
        // fallback for browsers that do not support the Clipboard API
        copyTextToClipboardFallback(text);
    }
}

function copyTextToClipboardFallback(text: string) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    try {
        document.execCommand("copy");
        console.log("Text copied to clipboard successfully.");
    } catch (err) {
        console.error("Failed to copy text using fallback method: ", err);
    }
    document.body.removeChild(textarea);
}

export function getParentPath(fullPath: string): string {
    const parts = fullPath.split("/").filter(Boolean);
    if (parts.length === 0) return "/";
    return "/" + parts.slice(0, -1).join("/");
}

export function getFileExtension(path: string): string {
    const parts = path.split(".");
    if (parts.length === 1) return "";
    return parts.pop() || "";
}

/**
 * Format Plex event names for display by removing the "media." prefix.
 * Examples: "media.scrobble" -> "scrobble", "media.rate" -> "rate"
 */
export function formatEventName(event?: string): string {
    if (!event) return "";
    // Remove "media." prefix if present
    if (event.startsWith("media.")) {
        return event.substring(6); // "media." is 6 characters
    }
    return event;
}