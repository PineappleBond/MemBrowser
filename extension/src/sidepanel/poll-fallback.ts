import { Update } from '../shared/types';
import { UpdateStore } from './update-store';

export class PollFallback {
  private timer: number | null = null;
  private interval: number = 1000;
  private maxInterval: number = 30000;
  private maxConsecutive: number = 10;
  private consecutive: number = 0;
  private running: boolean = false;
  private authKey: string = '';

  start(baseUrl: string, updateStore: UpdateStore, authKey?: string) {
    if (this.running) return;
    this.running = true;
    this.interval = 1000;
    this.consecutive = 0;
    this.authKey = authKey ?? this.authKey;
    console.log('[MB][poll] started, baseUrl:', baseUrl, 'interval:', this.interval, 'ms');
    this.poll(baseUrl, updateStore);
  }

  private async poll(baseUrl: string, updateStore: UpdateStore) {
    if (!this.running) return;

    const lastSeq = updateStore.getLastSeq();
    const url = `${baseUrl}/api/v1/updates?last_seq=${lastSeq}&limit=500`;
    console.log('[MB][poll] requesting, lastSeq:', lastSeq, 'consecutive:', this.consecutive);

    const headers: Record<string, string> = {};
    if (this.authKey) headers['Authorization'] = `Bearer ${this.authKey}`;

    try {
      const resp = await fetch(url, { headers });
      if (!resp.ok) {
        console.warn('[MB][poll] HTTP error:', resp.status, resp.statusText);
        this.interval = Math.min(this.interval * 2, this.maxInterval);
      } else {
        const data = await resp.json();
        const count = data.updates?.length || 0;
        console.log('[MB][poll] response, updates:', count, 'has_more:', data.has_more, 'max_seq:', data.max_seq);

        if (count > 0) {
          updateStore.applyUpdates(data.updates);
          this.interval = 1000;
          this.consecutive = 0;
        } else {
          this.interval = Math.min(this.interval * 2, this.maxInterval);
        }

        if (data.has_more && this.consecutive < this.maxConsecutive) {
          this.consecutive++;
          console.log('[MB][poll] has_more, fetching again immediately');
          this.poll(baseUrl, updateStore);
          return;
        }
      }
    } catch (e) {
      console.warn('[MB][poll] fetch error:', e);
      this.interval = Math.min(this.interval * 2, this.maxInterval);
    }

    if (this.running) {
      this.consecutive = 0;
      console.log('[MB][poll] next poll in', this.interval, 'ms');
      this.timer = window.setTimeout(() => this.poll(baseUrl, updateStore), this.interval);
    }
  }

  stop() {
    if (this.running) {
      console.log('[MB][poll] stopped');
    }
    this.running = false;
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}
