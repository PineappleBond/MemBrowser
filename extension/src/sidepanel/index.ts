import { UpdateStore } from './update-store';
import { WSClient } from './ws-client';
import { PollFallback } from './poll-fallback';
import { HttpClient } from './http-client';
import { Update } from '../shared/types';

// --- DOM ---
const statusDot = document.getElementById('statusDot')!;
const statusText = document.getElementById('statusText')!;
const taskInput = document.getElementById('taskInput') as HTMLTextAreaElement;
const submitBtn = document.getElementById('submitBtn')!;
const stopBtn = document.getElementById('stopBtn')!;
const settingsToggle = document.getElementById('settingsToggle')!;
const settingsSection = document.getElementById('settingsSection')!;
const saveSettingsBtn = document.getElementById('saveSettingsBtn')!;
const testConnBtn = document.getElementById('testConnBtn') as HTMLButtonElement;
const wsUrlInput = document.getElementById('wsUrl') as HTMLInputElement;
const authKeyInput = document.getElementById('authKey') as HTMLInputElement;
const baseUrlInput = document.getElementById('baseUrl') as HTMLInputElement;
const helpBtn = document.getElementById('helpBtn')!;
const toast = document.getElementById('toast')!;
const modelChips = document.querySelectorAll<HTMLButtonElement>('.model-chip');
const taskBadge = document.getElementById('taskBadge')!;
const stepList = document.getElementById('stepList')!;
const helpBanner = document.getElementById('helpBanner')!;
const helpText = document.getElementById('helpText')!;
const memoryIndicator = document.getElementById('memoryIndicator')!;
const thinkingArea = document.getElementById('thinkingArea')!;
const thinkingText = document.getElementById('thinkingText')!;

let selectedModel = 'sonnet';
let wsConnected = false;
let currentTaskId: string | null = null;

// --- HTTP Client ---
const httpClient = new HttpClient('http://localhost:8080', 'change-me');

// --- Toast ---
function showToast(msg: string, type: 'ok' | 'err' = 'ok') {
  console.log('[MB][ui] toast:', type, msg);
  toast.textContent = msg;
  toast.className = `toast show ${type}`;
  setTimeout(() => { toast.className = 'toast'; }, 2000);
}

// --- Task UI Rendering ---
function addStep(status: string, text: string) {
  const item = document.createElement('div');
  item.className = `step-item${status === 'running' ? ' active' : ''}`;
  item.innerHTML = `
    <div class="step-icon ${status}">
      ${status === 'completed' ? '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M5 13l4 4L19 7"/></svg>' :
        status === 'running' ? '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><circle cx="12" cy="12" r="3"/></svg>' :
        status === 'failed' ? '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M6 18L18 6M6 6l12 12"/></svg>' : ''}
    </div>
    <div class="step-content">
      <div class="step-text">${text}</div>
    </div>`;
  stepList.appendChild(item);
}

function clearSteps() {
  stepList.innerHTML = '';
}

function setTaskBadge(status: 'running' | 'completed' | 'failed' | null) {
  if (!status) {
    taskBadge.style.display = 'none';
    return;
  }
  taskBadge.style.display = '';
  taskBadge.className = `task-badge ${status}`;
  taskBadge.textContent = status === 'running' ? '运行中' : status === 'completed' ? '已完成' : '失败';
}

// --- 消息转发：Side Panel → Background → Content Script ---
async function forwardToActiveTab(message: any): Promise<any> {
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tabs.length || !tabs[0].id) {
    console.warn('[MB][ui] no active tab');
    return null;
  }
  const tab = tabs[0];
  // 检查是否为受限页面（content script 无法注入）
  const url = tab.url || '';
  if (!url || url.startsWith('chrome://') || url.startsWith('chrome-extension://') || url.startsWith('edge://') || url.startsWith('about:')) {
    console.warn('[MB][ui] active tab is restricted page, cannot inject content script. url:', url);
    throw new Error('当前页面不支持插件注入: ' + (url || '未知页面'));
  }
  const tabId = tab.id;
  console.log('[MB][ui] forwarding to tab:', tabId, 'url:', url);
  try {
    const resp = await chrome.runtime.sendMessage({
      type: 'forward_to_tab',
      tabId,
      frame: message,
    });
    if (!resp) {
      console.warn('[MB][ui] forwardToActiveTab returned empty response, content script may not be loaded');
      throw new Error('content script 未响应，请确保插件已加载（刷新页面重试）');
    }
    return resp;
  } catch (e) {
    console.error('[MB][ui] forwardToActiveTab failed:', e);
    throw e;
  }
}

// --- 处理 page.query：转发到 Content Script，上传结果 ---
async function handlePageQuery(payload: any) {
  const requestId = payload?.request_id;
  console.log('[MB][ui] handlePageQuery, request_id:', requestId);

  try {
    const resp = await forwardToActiveTab({ type: 'get_page_state' });
    await httpClient.uploadPageState({
      request_id: requestId,
      url: resp?.url || '',
      title: resp?.title || '',
      interactables: resp?.interactables || [],
      headings: resp?.headings || [],
    });
    console.log('[MB][ui] handlePageQuery done, request_id:', requestId);
  } catch (e) {
    console.error('[MB][ui] handlePageQuery failed:', e);
    // 不发送空数据，而是发送错误标记，让后端/agent知道获取失败
    addStep('failed', '获取页面失败: ' + (e instanceof Error ? e.message : '未知错误'));
  }
}

// --- 处理 action.execute：转发到 Content Script，上传结果 ---
async function handleActionExecute(payload: any) {
  const requestId = payload?.request_id;
  const action = payload?.action;
  const selector = payload?.selector;
  const value = payload?.value;
  console.log('[MB][ui] handleActionExecute, request_id:', requestId, 'action:', action);

  try {
    const resp = await forwardToActiveTab({
      type: 'execute_action',
      action,
      selector,
      value,
    });

    if (resp) {
      await httpClient.uploadActionResult({
        request_id: requestId,
        success: resp.success !== false,
        message: resp.error || '',
      });
    } else {
      await httpClient.uploadActionResult({
        request_id: requestId,
        success: false,
        error: '无法转发到目标标签页',
      });
    }
    console.log('[MB][ui] handleActionExecute done, request_id:', requestId);
  } catch (e) {
    console.error('[MB][ui] handleActionExecute failed:', e);
    try {
      await httpClient.uploadActionResult({
        request_id: requestId,
        success: false,
        error: String(e),
      });
    } catch (e2) {
      console.error('[MB][ui] handleActionExecute fallback upload also failed:', e2);
    }
  }
}

// --- 处理 need_help：显示示教提示 ---
function handleNeedHelp(payload: any) {
  helpBanner.style.display = '';
  helpText.textContent = payload?.question || payload?.info?.message || '需要你的帮助';
}

// --- 人类示教：用户点击帮助按钮后，收集当前页面信息并上传 ---
async function handleTeach() {
  console.log('[MB][ui] handleTeach: collecting page info');
  helpBanner.style.display = 'none';

  try {
    const pageResp = await forwardToActiveTab({ type: 'get_page_state' });
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const activeTab = tabs[0];

    await httpClient.uploadTeach({
      page_url: pageResp?.url || activeTab?.url || '',
      page_title: pageResp?.title || activeTab?.title || '',
      action_type: 'click',
      action_target: helpText.textContent || '',
    });
    showToast('示教数据已上传');
  } catch (e: any) {
    showToast('示教失败: ' + e.message, 'err');
    helpBanner.style.display = '';
  }
}

// --- thinking / message.delta 流式渲染 ---
let thinkingBuffer = '';
let messageBuffer = '';

function handleThinking(payload: any) {
  const delta = payload?.delta || '';
  if (delta) {
    thinkingBuffer += delta;
    thinkingArea.style.display = '';
    thinkingText.textContent = thinkingBuffer.slice(-500); // 只显示最后 500 字符
  }
}

function handleMessageDelta(payload: any) {
  const delta = payload?.delta || '';
  if (delta) {
    messageBuffer += delta;
    // 更新最后一个 step 的内容
    const items = stepList.querySelectorAll('.step-item');
    const last = items[items.length - 1];
    if (last) {
      const textEl = last.querySelector('.step-text');
      if (textEl) {
        textEl.textContent = messageBuffer.slice(-200);
      }
    }
  }
}

function resetStreamingBuffers() {
  thinkingBuffer = '';
  messageBuffer = '';
  thinkingArea.style.display = 'none';
  thinkingText.textContent = '';
}

// --- Update 渲染 ---
function renderUpdate(update: Update) {
  console.log('[MB][ui] renderUpdate:', update.type);
  switch (update.type) {
    case 'task.started':
      clearSteps();
      resetStreamingBuffers();
      setTaskBadge('running');
      submitBtn.style.display = 'none';
      stopBtn.style.display = '';
      currentTaskId = update.payload?.task_id || null;
      addStep('running', '任务开始执行...');
      break;
    case 'task.step':
      if (update.payload?.content) {
        // 把前一个 running 标记为 completed
        const items = stepList.querySelectorAll('.step-item');
        const last = items[items.length - 1];
        if (last) {
          const icon = last.querySelector('.step-icon');
          if (icon?.classList.contains('running')) {
            icon.className = 'step-icon completed';
            icon.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M5 13l4 4L19 7"/></svg>';
          }
        }
        addStep('running', update.payload.content);
        resetStreamingBuffers();
      }
      break;
    case 'task.completed':
      setTaskBadge('completed');
      submitBtn.style.display = '';
      stopBtn.style.display = 'none';
      // 把最后一个 running 标记为 completed
      const items2 = stepList.querySelectorAll('.step-item');
      const last2 = items2[items2.length - 1];
      if (last2) {
        const icon2 = last2.querySelector('.step-icon');
        if (icon2?.classList.contains('running')) {
          icon2.className = 'step-icon completed';
          icon2.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M5 13l4 4L19 7"/></svg>';
        }
      }
      addStep('completed', update.payload?.summary || '任务完成');
      showToast('任务完成');
      resetStreamingBuffers();
      break;
    case 'task.failed':
      setTaskBadge('failed');
      submitBtn.style.display = '';
      stopBtn.style.display = 'none';
      addStep('failed', '失败: ' + (update.payload?.error || '未知错误'));
      showToast('任务失败: ' + (update.payload?.error || ''), 'err');
      resetStreamingBuffers();
      break;
    case 'need_help':
      handleNeedHelp(update.payload);
      break;
    case 'thinking':
      handleThinking(update.payload);
      break;
    case 'message.delta':
      handleMessageDelta(update.payload);
      break;
    case 'page.query':
      addStep('running', '查询页面状态...');
      handlePageQuery(update.payload).catch(e => console.error('[MB][ui] handlePageQuery unhandled:', e));
      break;
    case 'action.execute':
      addStep('running', '执行操作: ' + (update.payload?.action || ''));
      handleActionExecute(update.payload).catch(e => console.error('[MB][ui] handleActionExecute unhandled:', e));
      break;
  }
}

// --- WebSocket + Poll Fallback ---
const updateStore = new UpdateStore(
  (_from, _to) => {
    console.warn('[MB] gap detected, triggering HTTP pull');
    // 当检测到 gap 时，立即通过 poll 拉取缺失的消息
    const baseUrl = baseUrlInput?.value ?? 'http://localhost:8080';
    const authKey = authKeyInput.value;
    pollFallback.start(baseUrl, updateStore, authKey);
  },
  renderUpdate
);
const wsClient = new WSClient(updateStore);
const pollFallback = new PollFallback();

wsClient.onStatusChange = (connected) => {
  wsConnected = connected;
  statusDot.className = connected ? 'status-dot connected' : 'status-dot';
  statusText.textContent = connected ? '已连接' : '未连接';
  if (!connected) {
    const baseUrl = baseUrlInput?.value ?? 'http://localhost:8080';
    pollFallback.start(baseUrl, updateStore, authKeyInput.value);
  } else {
    pollFallback.stop();
  }
};

async function init() {
  try {
    await updateStore.init();
    const config = await chrome.storage.local.get(['wsUrl', 'authKey', 'model', 'baseUrl']);
    wsUrlInput.value = config.wsUrl ?? 'ws://localhost:8080/ws';
    authKeyInput.value = config.authKey ?? 'change-me';
    if (baseUrlInput) baseUrlInput.value = config.baseUrl ?? 'http://localhost:8080';
    selectedModel = config.model ?? 'sonnet';
    modelChips.forEach(c => c.classList.toggle('active', c.dataset.model === selectedModel));

    httpClient.updateConfig(
      baseUrlInput?.value ?? 'http://localhost:8080',
      authKeyInput.value
    );

    wsClient.connect(wsUrlInput.value, authKeyInput.value);
  } catch (e: any) {
    console.error('[MB][ui] init failed:', e);
    showToast('初始化失败: ' + e.message, 'err');
  }
}

// --- Model Selector ---
modelChips.forEach(chip => {
  chip.addEventListener('click', () => {
    selectedModel = chip.dataset.model!;
    modelChips.forEach(c => c.classList.toggle('active', c === chip));
    chrome.storage.local.set({ model: selectedModel });
  });
});

// --- Task Submit ---
submitBtn.addEventListener('click', async () => {
  const task = taskInput.value.trim();
  if (!task) return;
  const baseUrl = baseUrlInput?.value ?? 'http://localhost:8080';
  const authKey = authKeyInput.value;
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (authKey) headers['Authorization'] = `Bearer ${authKey}`;
  try {
    const resp = await fetch(`${baseUrl}/api/v1/tasks`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ task, model: selectedModel })
    });
    if (!resp.ok) throw new Error(`${resp.status}`);
    taskInput.value = '';
    showToast('任务已提交');
  } catch (e: any) {
    showToast('提交失败: ' + e.message, 'err');
  }
});

// --- Stop Task ---
stopBtn.addEventListener('click', async () => {
  if (!currentTaskId) return;
  try {
    await httpClient.stopTask(currentTaskId);
    showToast('已发送停止指令');
  } catch (e: any) {
    showToast('停止失败: ' + e.message, 'err');
  }
});

// --- Settings Toggle ---
settingsToggle.addEventListener('click', (e) => {
  e.preventDefault();
  const computed = getComputedStyle(settingsSection).display;
  settingsSection.style.display = computed === 'none' ? 'block' : 'none';
});

// --- Save Settings ---
saveSettingsBtn.addEventListener('click', async () => {
  const settings = {
    wsUrl: wsUrlInput.value,
    authKey: authKeyInput.value,
    baseUrl: baseUrlInput?.value ?? 'http://localhost:8080',
  };
  await chrome.storage.local.set(settings);
  httpClient.updateConfig(settings.baseUrl, settings.authKey);
  showToast('已保存');
  wsClient.connect(settings.wsUrl, settings.authKey);
});

// --- Test Connection ---
testConnBtn.addEventListener('click', () => {
  if (wsConnected) {
    showToast('当前已连接', 'ok');
  } else {
    showToast('未连接，正在等待自动重连...', 'err');
  }
});

// --- Help Banner (示教) ---
helpBtn.addEventListener('click', handleTeach);

// --- 监听 Background 转发的 tab 事件 ---
chrome.runtime.onMessage.addListener((msg) => {
  if (msg.type === 'tab_event') {
    const event = msg.event;
    const payload = msg.payload;
    httpClient.reportTabEvent(event, payload);
  }
});

init();
