chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });
console.log('[MB][bg] service worker started');

function broadcast(message: object) {
  chrome.runtime.sendMessage(message).catch(() => {});
}

chrome.tabs.onCreated.addListener((tab) => {
  console.log('[MB][bg] tab created, id:', tab.id, 'url:', tab.url);
  broadcast({
    type: 'tab_event',
    event: 'opened',
    payload: { tab_id: tab.id, url: tab.url, title: tab.title }
  });
});

chrome.tabs.onRemoved.addListener((tabId) => {
  console.log('[MB][bg] tab removed, id:', tabId);
  broadcast({
    type: 'tab_event',
    event: 'closed',
    payload: { tab_id: tabId }
  });
});

chrome.tabs.onActivated.addListener((activeInfo) => {
  console.log('[MB][bg] tab activated, id:', activeInfo.tabId, 'windowId:', activeInfo.windowId);
  broadcast({
    type: 'tab_event',
    event: 'activated',
    payload: { tab_id: activeInfo.tabId }
  });
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'forward_to_tab' && msg.tabId != null && msg.frame != null) {
    console.log('[MB][bg] forward_to_tab, tabId:', msg.tabId, 'frame type:', msg.frame?.type);
    chrome.tabs.sendMessage(msg.tabId, msg.frame).then((resp) => {
      if (resp !== undefined) {
        console.log('[MB][bg] content script responded');
        sendResponse(resp);
      } else {
        // Content script might not be loaded — try programmatic injection
        console.log('[MB][bg] no response, attempting programmatic injection...');
        chrome.scripting.executeScript({
          target: { tabId: msg.tabId },
          files: ['content/index.js'],
        }).then(() => {
          // Retry after injection
          chrome.tabs.sendMessage(msg.tabId, msg.frame).then(sendResponse).catch((err) => {
            console.warn('[MB][bg] injection + retry failed:', err);
            sendResponse({ success: false, error: String(err) });
          });
        }).catch((injectErr) => {
          console.warn('[MB][bg] programmatic injection failed:', injectErr);
          sendResponse({ success: false, error: 'content script 未加载，且无法注入: ' + String(injectErr) });
        });
      }
    }).catch((err) => {
      console.warn('[MB][bg] forward_to_tab failed:', err);
      // Same fallback: try injection
      chrome.scripting.executeScript({
        target: { tabId: msg.tabId },
        files: ['content/index.js'],
      }).then(() => {
        chrome.tabs.sendMessage(msg.tabId, msg.frame).then(sendResponse).catch((err2) => {
          sendResponse({ success: false, error: '注入后仍失败: ' + String(err2) });
        });
      }).catch((injectErr) => {
        sendResponse({ success: false, error: '无法访问该页面: ' + String(injectErr) });
      });
    });
    return true;
  }
});
