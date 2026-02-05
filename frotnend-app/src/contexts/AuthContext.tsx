import { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { authService, User } from '@/services/auth';

interface AuthContextType {
  user: User | null;
  token: string | null;
  logout: () => void;
  isLoading: boolean;
  setUserAndToken: (user: User, token: string) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const initAuth = async () => {
      // Check for token in URL (from OAuth callback redirect)
      const urlParams = new URLSearchParams(window.location.search);
      const tokenFromUrl = urlParams.get('token');
      
      if (tokenFromUrl) {
        // Clear the token from URL to clean up
        window.history.replaceState({}, document.title, window.location.pathname);
        
        try {
          // Fetch user with the token
          const userData = await authService.getCurrentUser(tokenFromUrl);
          setToken(tokenFromUrl);
          setUser(userData);
          authService.storeAuth(tokenFromUrl, userData);
        } catch (error) {
          console.error('Failed to fetch user with token:', error);
        }
      } else {
        // Check for stored auth on mount
        const storedToken = authService.getStoredToken();
        const storedUser = authService.getStoredUser();

        if (storedToken && storedUser) {
          setToken(storedToken);
          setUser(storedUser);
        }
      }
      setIsLoading(false);
    };

    initAuth();
  }, []);

  const setUserAndToken = (newUser: User, newToken: string) => {
    setUser(newUser);
    setToken(newToken);
    authService.storeAuth(newToken, newUser);
  };

  const logout = () => {
    authService.logout();
    setToken(null);
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, token, logout, isLoading, setUserAndToken }}>
      {children}
    </AuthContext.Provider>
  );
};
