import { Update } from '../shared/types';

export type UpdateHandler = (update: Update) => void;

export class UpdateStore {
  private lastSeq: number = 0;
  private onGapDetected: (from: number, to: number) => void;
  private onUpdate: UpdateHandler;

  constructor(onGapDetected: (from: number, to: number) => void, onUpdate: UpdateHandler) {
    this.onGapDetected = onGapDetected;
    this.onUpdate = onUpdate;
  }

  async init() {
    const data = await chrome.storage.local.get(['lastSeq']);
    this.lastSeq = data.lastSeq ?? 0;
    console.log('[MB][store] init, lastSeq:', this.lastSeq);
  }

  getLastSeq(): number {
    return this.lastSeq;
  }

  applyUpdates(updates: Update[]): void {
    const ephemeral = updates.filter(u => u.seq === 0);
    const persisted = updates.filter(u => u.seq > 0).sort((a, b) => a.seq - b.seq);
    console.log('[MB][store] applyUpdates, total:', updates.length, 'ephemeral:', ephemeral.length, 'persisted:', persisted.length, 'lastSeq:', this.lastSeq);

    // 检查连续性（逐条递增 expectedSeq）
    let expectedSeq = this.lastSeq + 1;
    for (const update of persisted) {
      if (update.seq !== expectedSeq) {
        console.warn(`[MB][store] seq gap: expected ${expectedSeq}, got ${update.seq}, aborting batch`);
        this.onGapDetected(expectedSeq, update.seq - 1);
        return;
      }
      expectedSeq = update.seq + 1;
    }

    // 先处理 ephemeral (seq=0)
    for (const update of ephemeral) {
      this.processUpdate(update);
    }
    // 再按排序后的 persisted 顺序处理
    for (const update of persisted) {
      if (update.type === 'empty') {
        this.lastSeq = update.seq;
      } else {
        this.processUpdate(update);
        this.lastSeq = update.seq;
      }
    }

    this.persist();
  }

  private processUpdate(update: Update) {
    console.log('[MB][store] processUpdate:', update.type);
    this.onUpdate(update);
  }

  private async persist() {
    try {
      await chrome.storage.local.set({ lastSeq: this.lastSeq });
    } catch (e) {
      console.error('[MB][store] persist failed:', e);
    }
  }
}
