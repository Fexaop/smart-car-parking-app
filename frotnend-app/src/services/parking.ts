const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface Spot {
  id: string;
  number: number;
  occupied: boolean;
  otp?: string;
  otpExpiry?: string;
  lastOpen?: string;
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

export const parkingService = {
  listParks: async (): Promise<Park[]> => {
    const res = await fetch(`${API_BASE_URL}/api/parks`);
    if (!res.ok) throw new Error('failed to fetch parks');
    return res.json();
  },

  reserveSpot: async (parkId: string): Promise<ReserveResponse> => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/reserve`, { method: 'POST' });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'reserve failed');
    }
    return res.json();
  },
  
  reserveSpecific: async (parkId: string, spotId: string): Promise<ReserveResponse> => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/reserve`, { method: 'POST' });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'reserve failed');
    }
    return res.json();
  },

  releaseSpot: async (parkId: string, spotId: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/release`, { method: 'POST' });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'release failed');
    }
    return res.json();
  },

  validateOTP: async (parkId: string, spotId: string, otp: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/validate-otp`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ otp }),
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'validation failed');
    }
    return res.json();
  },

  validateOTPViaESP32: async (parkId: string, spotId: string, otp: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/validate-otp-esp32`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ otp }),
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'validation failed');
    }
    return res.json();
  },

  clearOTPDisplay: async (parkId: string, spotId: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/clear-otp-display`, {
      method: 'POST',
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'clear failed');
    }
    return res.json();
  },

  openGate: async (parkId: string, spotId: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/spots/${spotId}/opengate`, {
      method: 'POST',
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'open gate failed');
    }
    return res.json();
  },
};
