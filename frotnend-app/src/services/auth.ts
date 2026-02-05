export interface User {
  email: string;
  name: string;
  picture?: string;
}

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export const authService = {
  // Get the Google OAuth URL for iframe/webview
  getGoogleLoginUrl: () => {
    return `${API_BASE_URL}/login`;
  },

  // Handle OAuth callback
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
    const response = await fetch(`${API_BASE_URL}/protected`, {
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
};
