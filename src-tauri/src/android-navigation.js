(() => {
  const local = location.hostname === "tauri.localhost";
  const home = "http://tauri.localhost/android.html";
  const focusable =
    'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';
  const visible = (node) =>
    node instanceof HTMLElement &&
    !node.closest('[hidden], [inert], [aria-hidden="true"]') &&
    node.getClientRects().length > 0 &&
    getComputedStyle(node).visibility !== "hidden";
  const scope = () =>
    [...document.querySelectorAll('[aria-modal="true"]')]
      .filter(visible)
      .at(-1) || document;
  const controls = () =>
    [...scope().querySelectorAll(focusable)].filter(visible);
  const editable = (node) =>
    node instanceof HTMLElement &&
    (node.isContentEditable ||
      node.matches(
        'textarea, select, input:not([type="range"]):not([type="checkbox"]):not([type="radio"]):not([type="button"]):not([type="submit"])',
      ));

  window.__MUSIC_ANDROID_BACK__ = () => {
    const close = [...scope().querySelectorAll(".close-dialog")].find(visible);
    if (close) {
      close.click();
      return true;
    }
    if (document.querySelector(".player-tool-panel")) {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
      return true;
    }
    const songs = document.querySelector("#library-nav");
    if (songs && !songs.classList.contains("active")) {
      songs.click();
      songs.focus();
      return true;
    }
    if (!local) {
      location.replace(home);
      return true;
    }
    return false;
  };

  window.addEventListener(
    "keydown",
    (event) => {
      if (
        !event.key.startsWith("Arrow") ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        event.isComposing
      )
        return;
      // Capture first so the desktop player's arrows never steal TV navigation.
      event.stopImmediatePropagation();
      const active = document.activeElement;
      if (
        editable(active) &&
        !(
          active instanceof HTMLInputElement &&
          ["ArrowUp", "ArrowDown"].includes(event.key)
        )
      )
        return;
      if (
        active instanceof HTMLInputElement &&
        active.type === "range" &&
        ["ArrowLeft", "ArrowRight"].includes(event.key)
      )
        return;
      event.preventDefault();
      const items = controls();
      if (!items.length) return;
      if (!items.includes(active)) {
        items[0].focus();
        return;
      }
      const current = active.getBoundingClientRect();
      const x = current.x + current.width / 2,
        y = current.y + current.height / 2;
      const horizontal = ["ArrowLeft", "ArrowRight"].includes(event.key);
      const sign = ["ArrowLeft", "ArrowUp"].includes(event.key) ? -1 : 1;
      let best,
        score = Infinity;
      for (const item of items) {
        if (item === active) continue;
        const r = item.getBoundingClientRect();
        const dx = r.x + r.width / 2 - x,
          dy = r.y + r.height / 2 - y;
        const forward = (horizontal ? dx : dy) * sign;
        const sideways = Math.abs(horizontal ? dy : dx);
        if (forward <= 1) continue;
        const candidate = forward + sideways * 3;
        if (candidate < score) {
          best = item;
          score = candidate;
        }
      }
      if (best) {
        best.focus({ preventScroll: true });
        best.scrollIntoView({ block: "nearest", inline: "nearest" });
      }
    },
    true,
  );

  const ready = () => {
    // Constructed stylesheets preserve the Mac server's strict style-src policy.
    const style = new CSSStyleSheet();
    style.replaceSync(`:focus-visible { outline: 3px solid var(--accent, #b9dfb9) !important; outline-offset: 3px; }
      button, a, input, select { scroll-margin: 60px 8px; }
      .player { --control-size: 48px !important; }
      #player-window-btn, #pin-btn { display: none !important; }
      #music-android-bar { position:fixed; top:0; left:0; right:0; height:52px; z-index:1000; display:flex; align-items:center; justify-content:space-between; gap:12px; padding:0 12px; color:var(--text,#f0f3ed); background:var(--sidebar,#10191e); font:14px system-ui; }
      #music-android-bar button { min-height:48px; padding:8px 12px; color:inherit; background:transparent; border:0; font:inherit; }
      html[data-music-android-library] body { padding-top:52px; }
      html[data-music-android-library] .sidebar { top:52px; }
      html[data-music-android-library] .login-screen { min-height:calc(100dvh - 52px); }
      @media(pointer:coarse) { button, .nav-item { min-height:48px; min-width:48px; } }
    `);
    document.adoptedStyleSheets = [...document.adoptedStyleSheets, style];
    if (!local && location.protocol !== "about:") {
      document.documentElement.setAttribute("data-music-android-library", "");
      const bar = document.createElement("nav");
      bar.id = "music-android-bar";
      bar.setAttribute("aria-label", "Android client");
      const version = document.createElement("span");
      version.textContent = "Android · v" + window.__MUSIC_ANDROID_VERSION__;
      const back = document.createElement("button");
      back.textContent = "Connection";
      back.onclick = () => location.replace(home);
      bar.append(version, back);
      document.body.append(bar);
    }
    const focus = () => {
      if (!document.activeElement || document.activeElement === document.body)
        controls()[0]?.focus({ preventScroll: true });
    };
    focus();
    new MutationObserver(focus).observe(document.body, {
      childList: true,
      subtree: true,
    });
  };
  if (document.readyState === "loading")
    document.addEventListener("DOMContentLoaded", ready, { once: true });
  else ready();
})();
