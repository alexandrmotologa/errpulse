import { useEffect, useRef, useState } from 'react';

export function useErrPulseStream(projectId: number = 0, onEvent?: (event: any) => void) {
  const [connected, setConnected] = useState(false);
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    let es: EventSource | null = null;
    let timer: any = null;

    const connect = () => {
      const url = `/api/v1/stream?project_id=${projectId}`;
      es = new EventSource(url);

      es.onopen = () => {
        setConnected(true);
      };

      es.addEventListener('new_event', (e) => {
        try {
          const data = JSON.parse(e.data);
          onEventRef.current?.(data);
        } catch {
          // Ignore unparseable
        }
      });

      es.addEventListener('issue_update', (e) => {
        try {
          const data = JSON.parse(e.data);
          onEventRef.current?.(data);
        } catch {
          // Ignore unparseable
        }
      });

      es.onerror = () => {
        setConnected(false);
        es?.close();
        // Reconnect after 3 seconds
        timer = setTimeout(connect, 3000);
      };
    };

    connect();

    return () => {
      if (timer) clearTimeout(timer);
      es?.close();
    };
  }, [projectId]);

  return { connected };
}
