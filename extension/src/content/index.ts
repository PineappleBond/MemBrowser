interface InteractableElement {
  selector: string;
  tag: string;
  text: string;
  attributes: Record<string, string>;
  rect: { x: number; y: number; width: number; height: number };
}

interface PageState {
  url: string;
  title: string;
  interactables: InteractableElement[];
  headings: string[];
}

// 为元素生成唯一的 CSS selector
function generateSelector(el: Element): string {
  if (el.id) return `#${CSS.escape(el.id)}`;

  const parts: string[] = [];
  let current: Element | null = el;
  while (current && current !== document.body) {
    let selector = current.tagName.toLowerCase();
    if (current.id) {
      selector = `#${CSS.escape(current.id)}`;
      parts.unshift(selector);
      break;
    }
    const parent: Element | null = current.parentElement;
    if (parent) {
      const siblings = Array.from(parent.children).filter((c: Element) => c.tagName === current!.tagName);
      if (siblings.length > 1) {
        const index = siblings.indexOf(current) + 1;
        selector += `:nth-of-type(${index})`;
      }
    }
    parts.unshift(selector);
    current = parent;
  }
  return parts.join(' > ');
}

function isInteractable(el: Element): boolean {
  const tag = el.tagName.toLowerCase();
  if (['a', 'button', 'input', 'select', 'textarea'].includes(tag)) return true;
  if (el.getAttribute('role') === 'button' || el.getAttribute('role') === 'link') return true;
  if (el.getAttribute('onclick') || el.getAttribute('tabindex') === '0') return true;
  if (el.getAttribute('contenteditable') === 'true') return true;
  return false;
}

function isVisible(el: Element): boolean {
  const rect = el.getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0) return false;
  const style = getComputedStyle(el);
  if (style.display === 'none' || style.visibility === 'hidden') return false;
  return true;
}

function collectInteractables(): InteractableElement[] {
  const all = document.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [onclick], [tabindex="0"], [contenteditable="true"]');
  const result: InteractableElement[] = [];
  const seenSelectors = new Set<string>();

  for (const el of all) {
    if (result.length >= 500) break;
    if (!isVisible(el)) continue;

    const selector = generateSelector(el);
    if (seenSelectors.has(selector)) continue;
    seenSelectors.add(selector);

    const rect = el.getBoundingClientRect();
    const attrs: Record<string, string> = {};
    for (const attr of el.attributes) {
      if (['id', 'class', 'href', 'type', 'name', 'placeholder', 'value', 'aria-label', 'title'].includes(attr.name)) {
        attrs[attr.name] = attr.value.substring(0, 200);
      }
    }

    result.push({
      selector,
      tag: el.tagName.toLowerCase(),
      text: (el.textContent?.trim() || '').substring(0, 100),
      attributes: attrs,
      rect: { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) },
    });
  }

  return result;
}

function collectHeadings(): string[] {
  const headings = document.querySelectorAll('h1, h2, h3');
  return Array.from(headings).slice(0, 10).map(h => (h.textContent?.trim() || '').substring(0, 100));
}

function collectPageState(): PageState {
  return {
    url: window.location.href,
    title: document.title,
    interactables: collectInteractables(),
    headings: collectHeadings(),
  };
}

function setValueAndDispatch(el: Element, value: string) {
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
    el.value = value;
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  console.log('[MB][content] received message, type:', msg.type);
  try {
    if (msg.type === 'get_page_state') {
      const state = collectPageState();
      console.log('[MB][content] get_page_state, interactables:', state.interactables.length);
      sendResponse(state);
    } else if (msg.type === 'execute_action') {
      console.log('[MB][content] execute_action, action:', msg.action, 'selector:', msg.selector);
      let el: Element | null = null;
      try {
        el = document.querySelector(msg.selector);
      } catch (e) {
        sendResponse({ success: false, error: 'invalid selector: ' + msg.selector });
        return true;
      }
      if (!el) {
        sendResponse({ success: false, error: 'element not found: ' + msg.selector });
        return true;
      }
      switch (msg.action) {
        case 'click':
          (el as HTMLElement).click();
          sendResponse({ success: true });
          break;
        case 'input':
          setValueAndDispatch(el, msg.value || '');
          sendResponse({ success: true });
          break;
        case 'scroll':
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          sendResponse({ success: true });
          break;
        case 'navigate':
          window.location.href = msg.value || '';
          sendResponse({ success: true });
          break;
        default:
          sendResponse({ success: false, error: 'unknown action: ' + msg.action });
      }
    }
  } catch (e) {
    console.error('[MB][content] unhandled error:', e);
    sendResponse({ success: false, error: String(e) });
  }
  return true;
});

console.log('[MB][content] content script loaded, url:', window.location.href);
