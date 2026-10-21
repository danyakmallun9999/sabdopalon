// True only inside the Tauri webview — the same dashboard also runs in
// plain browsers, where the native OS chrome stays untouched.
export function isTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window
}
