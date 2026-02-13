import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { parkingService, Park, Spot, ReserveResponse } from "@/services/parking";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { User, Clock, Lock, Unlock } from "lucide-react";

interface ReservedSpot {
  spotId: string;
  parkId: string;
  parkName: string;
  spotNumber: number;
  otp: string;
  otpExpiry: string;
  otpInput: string;
  validated: boolean;
}

export default function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [parks, setParks] = useState<Park[]>([]);
  const [loading, setLoading] = useState(false);
  const [reservedSpots, setReservedSpots] = useState<Record<string, ReservedSpot>>({});
  const [busySpot, setBusySpot] = useState<string | null>(null);
  const [validatingSpot, setValidatingSpot] = useState<string | null>(null);

  const getInitials = (name: string) => {
    return name
      .split(' ')
      .map(n => n[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  const getTimeRemaining = (expiryStr: string): string => {
    const expiry = new Date(expiryStr);
    const now = new Date();
    const diff = expiry.getTime() - now.getTime();
    
    if (diff <= 0) return "Expired";
    
    const minutes = Math.floor(diff / 60000);
    const seconds = Math.floor((diff % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
  };

  const load = async () => {
    setLoading(true);
    try {
      const data = await parkingService.listParks();
      const sorted = data.sort((a, b) => {
        const aNum = parseInt(a.id.split('-')[1] || '0');
        const bNum = parseInt(b.id.split('-')[1] || '0');
        return aNum - bNum;
      });
      setParks(sorted);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { 
    load(); 
    
    // Update timers every second
    const interval = setInterval(() => {
      setReservedSpots(prev => ({ ...prev }));
    }, 1000);
    
    return () => clearInterval(interval);
  }, []);

  const handleReserve = async (parkId: string, spotId: string, parkName: string, spotNumber: number) => {
    try {
      setBusySpot(spotId);
      const res: ReserveResponse = await parkingService.reserveSpecific(parkId, spotId);
      
      // Update parks state
      setParks(prev => prev.map(p => p.id === parkId ? {
        ...p,
        spots: p.spots.map(s => s.id === spotId ? { ...s, occupied: true, otp: res.otp, otpExpiry: res.otpExpiry } : s)
      } : p));
      
      // Add to reserved spots
      setReservedSpots(prev => ({
        ...prev,
        [spotId]: {
          spotId,
          parkId,
          parkName,
          spotNumber,
          otp: res.otp,
          otpExpiry: res.otpExpiry,
          otpInput: '',
          validated: false,
        }
      }));
    } catch (err: any) {
      alert(err?.message || 'Failed to reserve spot');
    } finally {
      setBusySpot(null);
    }
  };

  const handleRelease = async (parkId: string, spotId: string) => {
    try {
      setBusySpot(spotId);
      await parkingService.releaseSpot(parkId, spotId);
      
      // Update parks state
      setParks(prev => prev.map(p => p.id === parkId ? {
        ...p,
        spots: p.spots.map(s => s.id === spotId ? { ...s, occupied: false, otp: undefined, otpExpiry: undefined } : s)
      } : p));
      
      // Remove from reserved spots
      setReservedSpots(prev => {
        const newSpots = { ...prev };
        delete newSpots[spotId];
        return newSpots;
      });
    } catch (err: any) {
      alert(err?.message || 'Failed to release spot');
    } finally {
      setBusySpot(null);
    }
  };

  const handleValidateOTP = async (spotId: string) => {
    const reserved = reservedSpots[spotId];
    if (!reserved) return;
    
    try {
      setValidatingSpot(spotId);
      await parkingService.validateOTP(reserved.parkId, spotId, reserved.otpInput);
      
      // Mark as validated
      setReservedSpots(prev => ({
        ...prev,
        [spotId]: { ...prev[spotId], validated: true }
      }));
      
      alert('Gate opened successfully! You can now control the gate.');
    } catch (err: any) {
      alert(err?.message || 'Invalid OTP');
    } finally {
      setValidatingSpot(null);
    }
  };

  const handleToggleGate = async (parkId: string, spotId: string) => {
    try {
      setBusySpot(spotId);
      await parkingService.openGate(parkId, spotId);
      alert('Gate toggle command sent!');
    } catch (err: any) {
      alert(err?.message || 'Failed to toggle gate');
    } finally {
      setBusySpot(null);
    }
  };

  const handleClearOTPDisplay = async (parkId: string, spotId: string) => {
    try {
      await parkingService.clearOTPDisplay(parkId, spotId);
      alert('OTP display cleared from LCD');
    } catch (err: any) {
      alert(err?.message || 'Failed to clear OTP display');
    }
  };

  return (
    <div className="min-h-screen p-6 bg-background">
      <div className="max-w-6xl mx-auto">
        {/* Header with Avatar */}
        <div className="flex justify-between items-center mb-6">
          <h1 className="text-2xl font-bold">Parking Dashboard</h1>
          <button 
            onClick={() => navigate('/profile')}
            className="flex items-center gap-2 hover:opacity-80 transition-opacity"
          >
            <span className="text-sm font-medium hidden sm:block">{user?.name}</span>
            <Avatar className="h-10 w-10 cursor-pointer border-2 border-primary">
              <AvatarImage src={user?.picture} alt={user?.name} />
              <AvatarFallback className="bg-primary text-primary-foreground">
                {user?.name ? getInitials(user.name) : <User className="h-5 w-5" />}
              </AvatarFallback>
            </Avatar>
          </button>
        </div>

        {loading && <p>Loading parks...</p>}

        {/* Parks Grid */}
        {parks.map((p) => (
          <div key={p.id} className="mb-6 p-4 border rounded-lg">
            <h2 className="font-semibold text-lg mb-3">{p.name}</h2>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
              {p.spots.map((s: Spot) => (
                <div key={s.id} className={`p-3 text-center rounded-lg shadow-sm ${s.occupied ? 'bg-red-600 text-white' : 'bg-emerald-600 text-white'}`}>
                  <div className="text-lg font-semibold">#{s.number}</div>
                  <div className="text-sm opacity-90 mb-2">{s.occupied ? 'Occupied' : 'Free'}</div>
                  {!s.occupied ? (
                    <Button 
                      type="button" 
                      size="sm" 
                      disabled={busySpot === s.id} 
                      onClick={() => handleReserve(p.id, s.id, p.name, s.number)}
                      className="w-full"
                    >
                      {busySpot === s.id ? 'Reserving...' : 'Reserve'}
                    </Button>
                  ) : (
                    <Button 
                      type="button" 
                      size="sm" 
                      variant="destructive" 
                      disabled={busySpot === s.id} 
                      onClick={() => handleRelease(p.id, s.id)}
                      className="w-full"
                    >
                      {busySpot === s.id ? 'Releasing...' : 'Release'}
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </div>
        ))}

        {/* Reserved Spots with OTP */}
        {Object.keys(reservedSpots).length > 0 && (
          <div className="mt-6">
            <h3 className="text-xl font-semibold mb-4">Your Reserved Spots</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {Object.entries(reservedSpots).map(([spotId, data]) => (
                <Card key={spotId} className={data.validated ? 'border-green-500 border-2' : ''}>
                  <CardHeader>
                    <CardTitle className="flex items-center justify-between">
                      <span>{data.parkName} - Spot #{data.spotNumber}</span>
                      {data.validated && <Lock className="h-5 w-5 text-green-500" />}
                    </CardTitle>
                    <CardDescription className="flex items-center gap-2">
                      <Clock className="h-4 w-4" />
                      {getTimeRemaining(data.otpExpiry)}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="p-3 bg-secondary/50 rounded-lg">
                      <p className="text-sm text-muted-foreground text-center">
                        Check the parking LCD screen for your OTP
                      </p>
                    </div>

                    {/* OTP Validation */}
                    {!data.validated && (
                      <div className="space-y-2">
                        <label className="text-sm font-medium">Enter OTP to access gate control:</label>
                        <div className="flex gap-2">
                          <Input
                            type="text"
                            placeholder="Enter 4-digit OTP"
                            maxLength={4}
                            value={data.otpInput}
                            onChange={(e) => setReservedSpots(prev => ({
                              ...prev,
                              [spotId]: { ...prev[spotId], otpInput: e.target.value }
                            }))}
                            className="text-center text-lg font-mono tracking-wider"
                          />
                          <Button
                            onClick={() => handleValidateOTP(spotId)}
                            disabled={validatingSpot === spotId || data.otpInput.length !== 4}
                          >
                            {validatingSpot === spotId ? 'Verifying...' : 'Verify'}
                          </Button>
                        </div>
                      </div>
                    )}

                    {/* Gate Control (after validation) */}
                    {data.validated && (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400 mb-2">
                          <Unlock className="h-4 w-4" />
                          <span>Access granted! Control the gate below:</span>
                        </div>
                        <div className="flex gap-2">
                          <Button
                            onClick={() => handleToggleGate(data.parkId, spotId)}
                            disabled={busySpot === spotId}
                            className="flex-1"
                          >
                            Toggle Gate
                          </Button>
                          <Button
                            variant="outline"
                            onClick={() => handleClearOTPDisplay(data.parkId, spotId)}
                          >
                            Clear LCD
                          </Button>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Toggle: Opens if closed, closes if open
                        </p>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        )}

        {Object.keys(reservedSpots).length === 0 && !loading && parks.length > 0 && (
          <div className="text-center text-muted-foreground py-8">
            <p>No reserved spots. Reserve a spot to get started!</p>
          </div>
        )}
      </div>
    </div>
  );
}
