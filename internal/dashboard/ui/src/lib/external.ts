// Opening links outside the desktop shell.
//
// The shell is a webview with no navigation bar, so a target="_blank" anchor
// there leads nowhere on its own. Two paths cover it:
//
//   1. The Tauri opener plugin injects its own click handler, which is the
//      primary path — capabilities/default.json grants opener:allow-open-url
//      for it. Without that grant the handler still ran, still called
//      preventDefault(), and then had its invoke rejected by the ACL: every
//      site URL in the dashboard looked like dead text.
//   2. This module, which the click interceptor in main.tsx uses. It is the
//      fallback for a page the injected script does not cover, and it cannot
//      double-open a link: the injected handler returns early when the event is
//      already defaultPrevented, and a listener on document runs before one on
//      window anyway.

type TauriGlobal = {
  core?: { invoke?: (cmd: string, args?: Record<string, unknown>) => Promise<unknown> }
  shell?: { open?: (path: string) => Promise<void> }
  opener?: { openUrl?: (url: string) => Promise<void> }
}

function tauri(): TauriGlobal | undefined {
  return (window as unknown as { __TAURI__?: TauriGlobal }).__TAURI__
}

/** inShell reports whether the dashboard is running inside the desktop app. */
export function inShell(): boolean {
  return Boolean(tauri())
}

/**
 * openExternal opens url in the OS browser (or a new tab outside the shell).
 * Resolves true when one of the paths accepted the request.
 */
export async function openExternal(url: string): Promise<boolean> {
  const t = tauri()
  if (!t) {
    // Plain browser: a new tab is the expected behaviour, and popup blockers
    // allow it because this runs on a user click.
    window.open(url, "_blank", "noopener")
    return true
  }
  // The plugin globals exist only once their injected api script has run, so
  // each of these is optional; the core invoke bridge below always exists.
  try {
    if (t.opener?.openUrl) {
      await t.opener.openUrl(url)
      return true
    }
  } catch {
    /* fall through to the next path */
  }
  try {
    if (t.shell?.open) {
      await t.shell.open(url)
      return true
    }
  } catch {
    /* fall through */
  }
  const invoke = t.core?.invoke
  if (!invoke) return false
  try {
    await invoke("plugin:opener|open_url", { url, with: null })
    return true
  } catch {
    /* fall through */
  }
  try {
    await invoke("plugin:shell|open", { path: url })
    return true
  } catch {
    return false
  }
}
