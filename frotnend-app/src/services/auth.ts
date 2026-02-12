import { invoke } from '@tauri-apps/api/core';
import { signIn } from '@choochmeque/tauri-plugin-google-auth-api';

export interface User {
  email: string;
  name: string;
  picture?: string;
}

// Helper to get API base URL from localStorage or environment
const getApiBaseUrl = (): string => {
  const savedIp = localStorage.getItem('serverIp');
  if (savedIp) {
    return `http://${savedIp}`;
  }
  return import.meta.env.VITE_API_URL || 'http://localhost:8080';
};

export const authService = {
  // Check if running on mobile platform
  isMobile: async (): Promise<boolean> => {
    try {
      return await invoke<boolean>('is_mobile');
    } catch (error) {
      console.error('Failed to check platform:', error);
      return false;
    }
  },

  // Google Auth for Android using the plugin
  googleAuthMobile: async (): Promise<{ user: User; token: string }> => {
    try {
      // Use the plugin's signIn function
      const result = await signIn({
        clientId: '97666001398-9trpf011q5tjfubotoador2bfbhknv4l.apps.googleusercontent.com',
        scopes: ['openid', 'email', 'profile'],
      });

      // Extract user info from ID token (you may need to decode the JWT)
      // For now, we'll use the tokens to authenticate with the backend
      
      // Exchange the Google ID token for our backend JWT
      const response = await fetch(`${getApiBaseUrl()}/auth/mobile`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          id_token: result.idToken,
          access_token: result.accessToken,
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to exchange token with backend');
      }

      const data = await response.json();
      
      // Decode the ID token to get user info (basic implementation)
      const idTokenPayload = JSON.parse(atob(result.idToken?.split('.')[1] || '{}'));
      
      const user: User = {
        email: idTokenPayload.email || '',
        name: idTokenPayload.name || '',
        picture: idTokenPayload.picture || '',
      };

      return { user, token: data.token };
    } catch (error) {
      console.error('Google auth failed:', error);
      throw error;
    }
  },

  // Get the Google OAuth URL for desktop (Go backend)
  getGoogleLoginUrl: () => {
    return `${getApiBaseUrl()}/login`;
  },

  // Handle OAuth callback (Desktop only)
  handleCallback: async (): Promise<void> => {
    const urlParams = new URLSearchParams(window.location.search);
    const token = urlParams.get('token');
    
    if (!token) {
      throw new Error('No token received from OAuth');
    }

    // Store token and fetch user info
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
      const timeoutId = setTimeout(() => controller.abort(), 5000); // 5 second timeout
      
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
