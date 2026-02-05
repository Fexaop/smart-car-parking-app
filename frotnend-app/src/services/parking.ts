const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface Spot {
  id: string;
  number: number;
  occupied: boolean;
  qrCode?: string;
}

export interface Park {
  id: string;
  name: string;
  spots: Spot[];
}

export const parkingService = {
  listParks: async (): Promise<Park[]> => {
    const res = await fetch(`${API_BASE_URL}/api/parks`);
    if (!res.ok) throw new Error('failed to fetch parks');
    return res.json();
  },

  reserveSpot: async (parkId: string) => {
    const res = await fetch(`${API_BASE_URL}/api/parks/${parkId}/reserve`, { method: 'POST' });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || 'reserve failed');
    }
    return res.json();
  },
  reserveSpecific: async (parkId: string, spotId: string) => {
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
};
