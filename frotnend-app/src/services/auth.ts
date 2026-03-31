import { invoke } from '@tauri-apps/api/core';

export interface User {
  email: string;
  name: string;
  picture?: string;
}

export interface MobileLoginLink {
  requestId: string;
  loginUrl: string;
  expiresIn: number;
}

const getApiBaseUrl = (): string => {
  const savedIp = localStorage.getItem('serverIp');
  if (savedIp) {
    return `http://${savedIp}`;
  }
  return import.meta.env.VITE_API_URL || 'http://localhost:8080';
};

const parseErrorResponse = async (response: Response, fallback: string): Promise<string> => {
  try {
    const text = (await response.text()).trim();
    return text || fallback;
  } catch {
    return fallback;
  }
};

export const authService = {
  isMobile: async (): Promise<boolean> => {
    try {
      return await invoke<boolean>('is_mobile');
    } catch (error) {
      console.error('Failed to check platform:', error);
      return false;
    }
  },

  createMobileLoginLink: async (): Promise<MobileLoginLink> => {
    const response = await fetch(`${getApiBaseUrl()}/auth/mobile/link`, {
      method: 'GET',
    });

    if (!response.ok) {
      const message = await parseErrorResponse(response, 'Failed to generate mobile login link');
      throw new Error(message);
    }

    const data = await response.json();
    if (!data.request_id || !data.login_url) {
      throw new Error('Invalid mobile login link response');
    }

    return {
      requestId: data.request_id,
      loginUrl: data.login_url,
      expiresIn: Number(data.expires_in || 0),
    };
  },

  verifyMobileOtp: async (requestId: string, otp: string): Promise<{ user: User; token: string }> => {
    const response = await fetch(`${getApiBaseUrl()}/auth/mobile/otp/verify`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        request_id: requestId,
        otp,
      }),
    });

    if (!response.ok) {
      const message = await parseErrorResponse(response, 'Invalid or expired OTP');
      throw new Error(message);
    }

    const data = await response.json();
    if (!data.token || !data.user) {
      throw new Error('Invalid OTP verification response');
    }

    return {
      token: data.token,
      user: data.user,
    };
  },

  getGoogleLoginUrl: () => {
    return `${getApiBaseUrl()}/login`;
  },

  handleCallback: async (): Promise<void> => {
    const urlParams = new URLSearchParams(window.location.search);
    const token = urlParams.get('token');
    
    if (!token) {
      throw new Error('No token received from OAuth');
    }

    localStorage.setItem('token', token);
    const user = await authService.getCurrentUser(token);
    localStorage.setItem('user', JSON.stringify(user));
  },

  getCurrentUser: async (token: string): Promise<User> => {
    const response = await fetch(`${getApiBaseUrl()}/protected`, {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      throw new Error('Failed to fetch user');
    }

    return response.json();
  },

  logout: () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  },

  getStoredToken: (): string | null => {
    return localStorage.getItem('token');
  },

  getStoredUser: (): User | null => {
    const userStr = localStorage.getItem('user');
    return userStr ? JSON.parse(userStr) : null;
  },

  storeAuth: (token: string, user: User) => {
    localStorage.setItem('token', token);
    localStorage.setItem('user', JSON.stringify(user));
  },

  testConnection: async (serverIp?: string): Promise<{ success: boolean; message: string }> => {
    const testUrl = serverIp ? `http://${serverIp}` : getApiBaseUrl();

    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000);

      await fetch(`${testUrl}/protected`, {
        method: 'OPTIONS',
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      return {
        success: true,
        message: `Successfully connected to ${testUrl}`,
      };
    } catch (error) {
      let message = 'Connection failed: ';

      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          message += 'Request timeout. Server not responding.';
        } else if (error.message.includes('Failed to fetch')) {
          message += 'Cannot reach server. Check if backend is running and IP is correct.';
        } else {
          message += error.message;
        }
      } else {
        message += 'Unknown error occurred';
      }

      return {
        success: false,
        message: `${message} (URL: ${testUrl})`,
      };
    }
  },
};
