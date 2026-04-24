/**
 * HTTP 客户端：向后端上传页面状态、操作结果、示教数据、标签页事件
 */
export class HttpClient {
  private baseUrl: string;
  private authKey: string;

  constructor(baseUrl: string, authKey: string) {
    this.baseUrl = baseUrl;
    this.authKey = authKey;
  }

  updateConfig(baseUrl: string, authKey: string) {
    this.baseUrl = baseUrl;
    this.authKey = authKey;
  }

  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.authKey) headers['Authorization'] = `Bearer ${this.authKey}`;
    return headers;
  }

  /** 上传页面状态（interactables + headings + URL + Title） */
  async uploadPageState(data: {
    request_id: string;
    url: string;
    title: string;
    interactables: any[];
    headings: string[];
    screenshot?: string;
  }): Promise<void> {
    console.log('[MB][http] uploadPageState, request_id:', data.request_id, 'url:', data.url, 'interactables:', data.interactables.length);
    const resp = await fetch(`${this.baseUrl}/api/v1/page/state`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(data),
    });
    if (!resp.ok) {
      console.error('[MB][http] uploadPageState failed:', resp.status);
    }
  }

  /** 上传操作执行结果 */
  async uploadActionResult(data: {
    request_id: string;
    success: boolean;
    message?: string;
    error?: string;
  }): Promise<void> {
    console.log('[MB][http] uploadActionResult, request_id:', data.request_id, 'success:', data.success);
    const resp = await fetch(`${this.baseUrl}/api/v1/action/result`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(data),
    });
    if (!resp.ok) {
      console.error('[MB][http] uploadActionResult failed:', resp.status);
    }
  }

  /** 上传人类示教数据 */
  async uploadTeach(data: {
    page_url: string;
    page_title?: string;
    action_type: string;
    action_target?: string;
    action_selector?: string;
    action_value?: string;
    screenshot?: string;
    dom_snapshot?: string;
  }): Promise<void> {
    console.log('[MB][http] uploadTeach, action_type:', data.action_type);
    const resp = await fetch(`${this.baseUrl}/api/v1/teach`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(data),
    });
    if (!resp.ok) {
      console.error('[MB][http] uploadTeach failed:', resp.status);
    }
  }

  /** 报告标签页事件 */
  async reportTabEvent(event: 'opened' | 'closed' | 'activated', payload: {
    tab_id?: number;
    url?: string;
    title?: string;
  }): Promise<void> {
    console.log('[MB][http] reportTabEvent:', event, payload);
    const resp = await fetch(`${this.baseUrl}/api/v1/tabs/${event}`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(payload),
    });
    if (!resp.ok) {
      console.error('[MB][http] reportTabEvent failed:', resp.status);
    }
  }

  /** 停止任务 */
  async stopTask(taskId: string): Promise<void> {
    console.log('[MB][http] stopTask:', taskId);
    const resp = await fetch(`${this.baseUrl}/api/v1/tasks/${taskId}/stop`, {
      method: 'POST',
      headers: this.getHeaders(),
    });
    if (!resp.ok) {
      console.error('[MB][http] stopTask failed:', resp.status);
    }
  }
}
