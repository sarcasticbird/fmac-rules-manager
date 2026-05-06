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

browser.browserAction.onClicked.addListener(() => {
  browser.runtime.openOptionsPage();
});

async function checkMACStatus() {
  try {
    const info = await browser.management.get(MAC_ID);
    return { ok: true, data: { enabled: info.enabled, name: info.name } };
  } catch (err) {
    return { ok: false, error: `Cannot check MAC status: ${err.message}` };
  }
}

browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    if (message.cmd === "check-mac") {
      sendResponse(await checkMACStatus());
      return;
    }

    const response = await sendToHost(message);
    sendResponse(response);
  })();

  return true;
});
