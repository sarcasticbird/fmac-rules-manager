const HOST_NAME = "fmac_rules_manager";
const MAC_ID = "@testpilot-containers";

let port = null;
let pendingCallbacks = [];

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

async function reloadMAC() {
  try {
    await browser.management.setEnabled(MAC_ID, false);
    await new Promise((r) => setTimeout(r, 500));
    await browser.management.setEnabled(MAC_ID, true);
    return { reloaded: true };
  } catch (err) {
    return { reloaded: false, warning: `Could not reload MAC: ${err.message}. Restart browser to apply changes.` };
  }
}

const WRITE_COMMANDS = new Set(["add", "update", "delete", "import"]);

browser.browserAction.onClicked.addListener(() => {
  browser.runtime.openOptionsPage();
});

browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    const response = await sendToHost(message);

    if (response.ok && WRITE_COMMANDS.has(message.cmd)) {
      const reload = await reloadMAC();
      if (response.data && typeof response.data === "object") {
        response.data._macReload = reload;
      }
    }

    sendResponse(response);
  })();

  return true;
});
