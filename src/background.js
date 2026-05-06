const HOST_NAME = "fmac_rules_manager";
const MAC_ID = "@testpilot-containers";

let port = null;
let pendingCallbacks = [];
let sessionActive = false;

function connectHost() {
  if (port) return port;
  port = browser.runtime.connectNative(HOST_NAME);

  port.onMessage.addListener((response) => {
    const cb = pendingCallbacks.shift();
    if (cb) cb(response);
  });

  port.onDisconnect.addListener((p) => {
    const error = p.error ? p.error.message : browser.runtime.lastError?.message || "disconnected";
    port = null;
    while (pendingCallbacks.length > 0) {
      const cb = pendingCallbacks.shift();
      cb({ ok: false, error: `Host disconnected: ${error}` });
    }
  });

  return port;
}

function sendToHost(message) {
  return new Promise((resolve) => {
    try {
      const p = connectHost();
      p.postMessage(message);
      pendingCallbacks.push(resolve);
    } catch (err) {
      resolve({
        ok: false,
        error: `Could not connect to native host. Is it installed? (${err.message})`,
      });
    }
  });
}

async function beginSession() {
  if (sessionActive) return { ok: true };
  try {
    await browser.management.setEnabled(MAC_ID, false);
    await new Promise((r) => setTimeout(r, 300));
    sessionActive = true;
    return { ok: true };
  } catch (err) {
    return { ok: false, error: `Could not disable MAC to release DB lock: ${err.message}` };
  }
}

async function endSession() {
  if (!sessionActive) return;
  sessionActive = false;
  try {
    await browser.management.setEnabled(MAC_ID, true);
  } catch (err) {
    // Best effort — MAC will reload on browser restart
  }
}

browser.browserAction.onClicked.addListener(() => {
  browser.runtime.openOptionsPage();
});

browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    if (message.cmd === "begin-session") {
      sendResponse(await beginSession());
      return;
    }

    if (message.cmd === "end-session") {
      await endSession();
      sendResponse({ ok: true });
      return;
    }

    const response = await sendToHost(message);
    sendResponse(response);
  })();

  return true;
});

// Re-enable MAC if the extension is unloaded or browser shuts down
browser.runtime.onSuspend?.addListener(() => {
  endSession();
});
