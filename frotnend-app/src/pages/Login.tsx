import { useTheme } from "@/contexts/ThemeContext";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Moon, Sun } from "lucide-react";
import { authService } from "@/services/auth";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

export default function Login() {
  const { theme, toggleTheme } = useTheme();
  const { setUserAndToken } = useAuth();
  const navigate = useNavigate();
  const [isMobile, setIsMobile] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [isVerifyingOtp, setIsVerifyingOtp] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mobileLoginLink, setMobileLoginLink] = useState<string | null>(null);
  const [mobileRequestId, setMobileRequestId] = useState<string | null>(null);
  const [mobileOtp, setMobileOtp] = useState("");
  const [linkExpiresIn, setLinkExpiresIn] = useState<number | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [serverIp, setServerIp] = useState(() => {
    return localStorage.getItem('serverIp') || '192.168.240.1:8080';
  });

  useEffect(() => {
    authService.isMobile().then(setIsMobile);
  }, []);

  const handleTestConnection = async () => {
    setIsTesting(true);
    setConnectionStatus(null);
    setError(null);

    const trimmedIp = serverIp.trim();
    if (!trimmedIp) {
      setConnectionStatus({ type: 'error', message: 'Please enter a server IP address' });
      setIsTesting(false);
      return;
    }

    const result = await authService.testConnection(trimmedIp);
    setConnectionStatus({
      type: result.success ? 'success' : 'error',
      message: result.message,
    });
    setIsTesting(false);

    if (result.success) {
      localStorage.setItem('serverIp', trimmedIp);
    }
  };

  const handleGoogleLogin = async () => {
    setIsLoading(true);
    setError(null);
    setConnectionStatus(null);

    const trimmedIp = serverIp.trim();
    if (!trimmedIp) {
      setError('Please enter a server IP address');
      setIsLoading(false);
      return;
    }

    localStorage.setItem('serverIp', trimmedIp);

    try {
      if (isMobile) {
        const linkData = await authService.createMobileLoginLink();
        setMobileRequestId(linkData.requestId);
        setMobileLoginLink(linkData.loginUrl);
        setLinkExpiresIn(linkData.expiresIn);
        setMobileOtp("");
      } else {
        window.location.href = authService.getGoogleLoginUrl();
      }
    } catch (err) {
      console.error('Login failed:', err);

      let errorMessage = 'Failed to sign in: ';
      if (err instanceof Error) {
        errorMessage += err.message;
        console.error('Error details:', {
          name: err.name,
          message: err.message,
          stack: err.stack,
        });
      } else {
        errorMessage += 'Unknown error';
        console.error('Error object:', err);
      }

      setError(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  const handleOpenMobileLink = () => {
    if (!mobileLoginLink) {
      return;
    }
    window.open(mobileLoginLink, '_blank', 'noopener,noreferrer');
  };

  const handleCopyMobileLink = async () => {
    if (!mobileLoginLink) {
      return;
    }

    try {
      await navigator.clipboard.writeText(mobileLoginLink);
      setConnectionStatus({ type: 'success', message: 'Login link copied to clipboard' });
    } catch {
      setConnectionStatus({ type: 'error', message: 'Could not copy the login link' });
    }
  };

  const handleVerifyOtp = async () => {
    setError(null);

    if (!mobileRequestId) {
      setError('Generate a login link first');
      return;
    }

    const otp = mobileOtp.trim();
    if (!otp) {
      setError('Enter the OTP from the browser');
      return;
    }

    setIsVerifyingOtp(true);
    try {
      const { user, token } = await authService.verifyMobileOtp(mobileRequestId, otp);
      setUserAndToken(user, token);
      navigate('/dashboard');
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('OTP verification failed');
      }
    } finally {
      setIsVerifyingOtp(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="absolute top-4 right-4">
        <Button variant="ghost" size="icon" onClick={toggleTheme} className="rounded-full">
          {theme === 'dark' ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
        </Button>
      </div>
      <Card className="w-full max-w-md border-border/70 bg-card/90 backdrop-blur">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold text-center">Welcome to Car Parking</CardTitle>
          <CardDescription className="text-center">
            Sign in to continue {isMobile ? '(Mobile)' : '(Desktop)'}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && (
            <div className="rounded-md border border-rose-500/40 bg-rose-500/10 p-3 text-sm text-rose-200">
              {error}
            </div>
          )}
          {connectionStatus && (
            <div className={`p-3 text-sm rounded-md ${
              connectionStatus.type === 'success' 
                ? 'border border-cyan-400/40 bg-cyan-500/10 text-cyan-200'
                : 'border border-amber-400/40 bg-amber-500/10 text-amber-200'
            }`}>
              {connectionStatus.message}
            </div>
          )}
          <div className="space-y-2">
            <label htmlFor="serverIp" className="text-sm font-medium text-foreground">
              Server Address
            </label>
            <div className="flex gap-2">
              <Input
                id="serverIp"
                type="text"
                placeholder="e.g., 192.168.240.1:8080"
                value={serverIp}
                onChange={(e) => setServerIp(e.target.value)}
                disabled={isLoading || isTesting}
                className="flex-1"
              />
              <Button
                type="button"
                variant="secondary"
                onClick={handleTestConnection}
                disabled={isLoading || isTesting}
                className="whitespace-nowrap"
              >
                {isTesting ? 'Testing...' : 'Test'}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {isMobile 
                ? 'Enter your backend server IP (Waydroid: 192.168.240.1:8080, Emulator: 10.0.2.2:8080)' 
                : 'Enter your backend server address (default: localhost:8080)'}
            </p>
          </div>
          {isMobile ? (
            <>
              <Button
                onClick={handleGoogleLogin}
                variant="outline"
                className="w-full flex items-center justify-center gap-2 h-11"
                disabled={isLoading || isVerifyingOtp}
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24">
                  <path fill="currentColor" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                  <path fill="currentColor" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="currentColor" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="currentColor" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                {isLoading ? 'Generating login link...' : 'Generate Mobile Login Link'}
              </Button>

              {mobileLoginLink && (
                <div className="space-y-3 rounded-md border border-border p-3">
                  <p className="text-sm text-foreground">1. Open this link in your browser and complete Google login.</p>
                  <a
                    href={mobileLoginLink}
                    target="_blank"
                    rel="noreferrer"
                    className="block break-all text-sm text-cyan-300 underline"
                  >
                    {mobileLoginLink}
                  </a>
                  <div className="flex gap-2">
                    <Button type="button" variant="secondary" onClick={handleOpenMobileLink} className="flex-1">
                      Open in Browser
                    </Button>
                    <Button type="button" variant="secondary" onClick={handleCopyMobileLink} className="flex-1">
                      Copy Link
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">Request ID: {mobileRequestId}</p>
                  {linkExpiresIn !== null && (
                    <p className="text-xs text-muted-foreground">Link expires in {Math.max(1, Math.floor(linkExpiresIn / 60))} minutes.</p>
                  )}
                </div>
              )}

              <div className="space-y-2">
                <label htmlFor="otp" className="text-sm font-medium text-foreground">
                  2. Enter OTP from browser
                </label>
                <Input
                  id="otp"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  placeholder="6-digit OTP"
                  value={mobileOtp}
                  onChange={(e) => setMobileOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  disabled={isVerifyingOtp || isLoading}
                />
                <Button
                  type="button"
                  onClick={handleVerifyOtp}
                  disabled={isVerifyingOtp || !mobileRequestId || mobileOtp.trim().length < 6}
                  className="w-full"
                >
                  {isVerifyingOtp ? 'Verifying OTP...' : 'Login with OTP'}
                </Button>
              </div>
            </>
          ) : (
            <Button 
              onClick={handleGoogleLogin} 
              variant="outline" 
              className="w-full flex items-center justify-center gap-2 h-11"
              disabled={isLoading}
            >
              <svg className="w-5 h-5" viewBox="0 0 24 24">
                <path fill="currentColor" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                <path fill="currentColor" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                <path fill="currentColor" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                <path fill="currentColor" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
              </svg>
              {isLoading ? 'Signing in...' : 'Continue with Google'}
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
