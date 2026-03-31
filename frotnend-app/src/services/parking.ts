const getApiBaseUrl = (): string => {
  const savedIp = localStorage.getItem('serverIp');
  if (savedIp) {
    return `http://${savedIp}`;
  }
  return import.meta.env.VITE_API_URL || 'http://localhost:8080';
};

export interface Spot {
  id: string;
  number: number;
  occupied: boolean;
  otp?: string;
  otpExpiry?: string;
  lastOpen?: string;
  ownerUserId?: string;
  ownedByCurrentUser?: boolean;
}

export interface Park {
  id: string;
  name: string;
  spots: Spot[];
}

export interface ReserveResponse {
  parkId: string;
  spotId: string;
  spotNumber: number;
  otp: string;
  otpExpiry: string;
}

export interface ParkingEvent {
  type: string;
  parkId: string;
  spot: Spot;
  actorIsCurrentUser?: boolean;
  timestamp?: string;
}

const getAuthToken = (): string | null => {
  return localStorage.getItem('token');
};

const getParkingWebSocketUrl = (): string => {
  const baseUrl = new URL(getApiBaseUrl());
  const protocol = baseUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  const token = getAuthToken();
  const tokenQuery = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${protocol}//${baseUrl.host}/ws/parking${tokenQuery}`;
};

const buildHeaders = (hasBody = false): HeadersInit => {
  const token = getAuthToken();
  const headers: Record<string, string> = {};

  if (hasBody) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return headers;
};

const parseError = async (res: Response, fallback: string): Promise<string> => {
  try {
    const text = (await res.text()).trim();
    return text || fallback;
  } catch {
    return fallback;
  }
};

const requestJson = async <T>(path: string, options?: RequestInit): Promise<T> => {
  const hasBody = Boolean(options?.body);
  const res = await fetch(`${getApiBaseUrl()}${path}`, {
    ...options,
    headers: {
      ...buildHeaders(hasBody),
      ...(options?.headers || {}),
    },
  });

  if (!res.ok) {
    if (res.status === 401) {
      throw new Error('Session expired. Please login again.');
    }
    throw new Error(await parseError(res, 'Request failed'));
  }

  return res.json();
};

export const parkingService = {
  listParks: async (): Promise<Park[]> => {
    return requestJson<Park[]>('/api/parks', { method: 'GET' });
  },

  reserveSpot: async (parkId: string): Promise<ReserveResponse> => {
    return requestJson<ReserveResponse>(`/api/parks/${parkId}/reserve`, { method: 'POST' });
  },

  reserveSpecific: async (parkId: string, spotId: string): Promise<ReserveResponse> => {
    return requestJson<ReserveResponse>(`/api/parks/${parkId}/spots/${spotId}/reserve`, { method: 'POST' });
  },

  releaseSpot: async (parkId: string, spotId: string) => {
    return requestJson<{ ok: boolean }>(`/api/parks/${parkId}/spots/${spotId}/release`, { method: 'POST' });
  },

  validateOTP: async (parkId: string, spotId: string, otp: string) => {
    return requestJson<{ ok: boolean; message: string }>(`/api/parks/${parkId}/spots/${spotId}/validate-otp`, {
      method: 'POST',
      body: JSON.stringify({ otp }),
    });
  },

  validateOTPViaESP32: async (parkId: string, spotId: string, otp: string) => {
    return requestJson<{ ok: boolean; message: string }>(`/api/parks/${parkId}/spots/${spotId}/validate-otp-esp32`, {
      method: 'POST',
      body: JSON.stringify({ otp }),
    });
  },

  clearOTPDisplay: async (parkId: string, spotId: string) => {
    return requestJson<{ ok: boolean; message: string }>(`/api/parks/${parkId}/spots/${spotId}/clear-otp-display`, {
      method: 'POST',
    });
  },

  openGate: async (parkId: string, spotId: string) => {
    return requestJson<{ ok: boolean; message: string }>(`/api/parks/${parkId}/spots/${spotId}/opengate`, {
      method: 'POST',
    });
  },

  subscribeParkingEvents: (
    onEvent: (event: ParkingEvent) => void,
    onConnectionChange?: (connected: boolean) => void,
  ) => {
    let socket: WebSocket | null = null;
    let disposed = false;
    let reconnectTimer: number | undefined;

    const clearReconnect = () => {
      if (reconnectTimer !== undefined) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = undefined;
      }
    };

    const connect = () => {
      if (disposed) {
        return;
      }

      if (!getAuthToken()) {
        onConnectionChange?.(false);
        return;
      }

      socket = new WebSocket(getParkingWebSocketUrl());

      socket.onopen = () => {
        onConnectionChange?.(true);
      };

      socket.onmessage = (event) => {
        if (typeof event.data !== 'string') {
          return;
        }

        try {
          const parsed = JSON.parse(event.data) as ParkingEvent;
          if (!parsed || !parsed.type || !parsed.parkId || !parsed.spot) {
            return;
          }
          onEvent(parsed);
        } catch {
          return;
        }
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onclose = () => {
        onConnectionChange?.(false);
        if (disposed) {
          return;
        }
        clearReconnect();
        reconnectTimer = window.setTimeout(connect, 1500);
      };
    };

    connect();

    return () => {
      disposed = true;
      clearReconnect();
      onConnectionChange?.(false);
      if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        socket.close();
      }
    };
  },
};
