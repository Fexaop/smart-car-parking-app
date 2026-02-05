import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { authService } from '@/services/auth';

export default function Callback() {
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const { setUserAndToken } = useAuth();

  useEffect(() => {
    const handleCallback = async () => {
      try {
        // Get token from URL
        const urlParams = new URLSearchParams(window.location.search);
        const token = urlParams.get('token');

        if (!token) {
          setError('No authentication token received');
          setTimeout(() => navigate('/login'), 2000);
          return;
        }

        // Fetch user info with the token
        const user = await authService.getCurrentUser(token);
        
        // Store auth and update context
        setUserAndToken(user, token);
        
        // Redirect to home
        navigate('/home');
      } catch (err) {
        console.error('Callback error:', err);
        setError('Authentication failed. Redirecting...');
        setTimeout(() => navigate('/login'), 2000);
      }
    };

    handleCallback();
  }, [navigate, setUserAndToken]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center">
        {error ? (
          <>
            <div className="text-destructive text-lg mb-4">{error}</div>
            <div className="text-muted-foreground">Redirecting to login...</div>
          </>
        ) : (
          <>
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent mb-4"></div>
            <p className="text-foreground">Completing authentication...</p>
          </>
        )}
      </div>
    </div>
  );
}
