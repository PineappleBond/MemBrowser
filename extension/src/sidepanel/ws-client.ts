import { Frame, ConnectedPayload, Update } from '../shared/types';
import { UpdateStore } from './update-store';

export class WSClient {
  private ws: WebSocket | null = null;
  private updateStore: UpdateStore;
  private reconnectDelay: number = 1000;
  private maxReconnectDelay: number = 30000;
  private timeoutTimer: number | null = null;
  onStatusChange?: (connected: boolean) => void;

  constructor(updateStore: UpdateStore) {
    this.updateStore = updateStore;
  }

  connect(url: string, authKey: string) {
    // 关闭旧连接，避免泄漏
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }
    this.stopTimers();
    this.reconnectDelay = 1000;

    const lastSeq = this.updateStore.getLastSeq();
    const fullUrl = `${url}?token=${encodeURIComponent(authKey)}&last_seq=${lastSeq}`;
    console.log('[MB][ws] connecting:', url, 'lastSeq:', lastSeq);
    try {
      this.ws = new WebSocket(fullUrl);
    } catch (err) {
      console.error('[MB][ws] failed to create WebSocket:', err);
      this.onStatusChange?.(false);
      setTimeout(() => this.connect(url, authKey), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
      return;
    }

    this.ws.onopen = () => {
      console.log('[MB][ws] connected, resetting reconnect delay');
      this.reconnectDelay = 1000;
      this.resetTimeout();
      this.onStatusChange?.(true);
    };

    this.ws.onmessage = (e) => {
      try {
        const frame = JSON.parse(e.data);
        console.log('[MB][ws] recv frame:', frame.type, frame.payload ? '' : '(no payload)');
        this.handleFrame(frame);
      } catch (err) {
        console.warn('[MB][ws] invalid message:', e.data, err);
      }
    };

    this.ws.onclose = (e) => {
      console.log('[MB][ws] closed, code:', e.code, 'reason:', e.reason, 'wasClean:', e.wasClean);
      this.stopTimers();
      this.onStatusChange?.(false);
      console.log('[MB][ws] will reconnect in', this.reconnectDelay, 'ms');
      setTimeout(() => this.connect(url, authKey), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };

    this.ws.onerror = (e) => {
      console.error('[MB][ws] error:', e);
      this.ws?.close();
    };
  }

  private handleFrame(frame: Frame) {
    switch (frame.type) {
      case 'connected': {
        const payload = frame.payload as ConnectedPayload;
        console.log('[MB][ws] server connected, session:', payload.session_id, 'maxSeq:', payload.max_seq);
        break;
      }
      case 'ping':
        console.log('[MB][ws] ping received, sending pong');
        this.send({ type: 'pong' });
        this.resetTimeout();
        break;
      case 'updates': {
        const updates = frame.payload as Update[];
        console.log('[MB][ws] updates received, count:', updates.length);
        this.updateStore.applyUpdates(updates);
        this.resetTimeout();
        break;
      }
      default:
        console.warn('[MB][ws] unknown frame type:', frame.type);
        break;
    }
  }

  private resetTimeout() {
    if (this.timeoutTimer) clearTimeout(this.timeoutTimer);
    this.timeoutTimer = window.setTimeout(() => {
      console.warn('[MB][ws] server timeout (60s no message), closing');
      this.ws?.close();
    }, 60000);
  }

  private stopTimers() {
    if (this.timeoutTimer) {
      clearTimeout(this.timeoutTimer);
      this.timeoutTimer = null;
    }
  }

  private send(data: any) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }
}
