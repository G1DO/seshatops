import type { ProjectionUpdated } from "../api/types";
import { EVENT_PROJECTION_UPDATED } from "../api/sse";

type Listener = (event: Event) => void;

/** Controllable EventSource stand-in for Vitest. */
export class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  readyState = 0;
  onopen: ((ev: Event) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  private readonly listeners = new Map<string, Set<Listener>>();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
    this.readyState = 1;
    // Fire after the caller assigns onopen (mirrors browser EventSource).
    queueMicrotask(() => {
      if (this.readyState === 1) {
        this.onopen?.(new Event("open"));
      }
    });
  }

  addEventListener(type: string, listener: Listener): void {
    const set = this.listeners.get(type) ?? new Set();
    set.add(listener);
    this.listeners.set(type, set);
  }

  removeEventListener(type: string, listener: Listener): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.readyState = 2;
  }

  emitUpdate(update: ProjectionUpdated): void {
    const event = new MessageEvent(EVENT_PROJECTION_UPDATED, {
      data: JSON.stringify(update),
    });
    for (const listener of this.listeners.get(EVENT_PROJECTION_UPDATED) ?? []) {
      listener(event);
    }
  }

  emitMalformed(): void {
    const event = new MessageEvent(EVENT_PROJECTION_UPDATED, {
      data: "{not-json",
    });
    for (const listener of this.listeners.get(EVENT_PROJECTION_UPDATED) ?? []) {
      listener(event);
    }
  }

  triggerError(): void {
    const event = new Event("error");
    this.onerror?.(event);
  }

  static reset(): void {
    MockEventSource.instances = [];
  }

  static latest(): MockEventSource {
    const last = MockEventSource.instances[MockEventSource.instances.length - 1];
    if (!last) {
      throw new Error("no MockEventSource instances");
    }
    return last;
  }
}

export function mockEventSourceFactory(url: string): EventSource {
  return new MockEventSource(url) as unknown as EventSource;
}
