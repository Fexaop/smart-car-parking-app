import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { parkingService, Park, Spot } from "@/services/parking";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { User } from "lucide-react";

export default function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [parks, setParks] = useState<Park[]>([]);
  const [loading, setLoading] = useState(false);
  const [qrCodes, setQrCodes] = useState<Record<string, { qr: string; parkName: string; spotNumber: number }>>({});
  const [busySpot, setBusySpot] = useState<string | null>(null);

  const getInitials = (name: string) => {
    return name
      .split(' ')
      .map(n => n[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  const load = async () => {
    setLoading(true);
    try {
      const data = await parkingService.listParks();
      // Sort parks by ID to ensure correct order (park-1, park-2, park-3, park-4)
      const sorted = data.sort((a, b) => {
        const aNum = parseInt(a.id.split('-')[1] || '0');
        const bNum = parseInt(b.id.split('-')[1] || '0');
        return aNum - bNum;
      });
      setParks(sorted);
      
      // Extract QR codes from occupied spots
      const qrs: Record<string, { qr: string; parkName: string; spotNumber: number }> = {};
      sorted.forEach(park => {
        park.spots.forEach(spot => {
          if (spot.occupied && spot.qrCode) {
            qrs[spot.id] = {
              qr: spot.qrCode,
              parkName: park.name,
              spotNumber: spot.number
            };
          }
        });
      });
      setQrCodes(qrs);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  return (
    <div className="min-h-screen p-6 bg-background">
      <div className="max-w-4xl mx-auto">
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

        {parks.map((p) => (
          <div key={p.id} className="mb-6 p-4 border rounded">
            <h2 className="font-semibold">{p.name}</h2>
            <div className="mt-2 grid grid-cols-5 gap-3">
              {p.spots.map((s: Spot) => (
                <div key={s.id} className={`p-3 text-center rounded-lg shadow-sm ${s.occupied ? 'bg-red-600 text-white' : 'bg-emerald-600 text-white'}`}>
                  <div className="text-lg font-semibold">#{s.number}</div>
                  <div className="text-sm opacity-90">{s.occupied ? 'Occupied' : 'Free'}</div>
                  <div className="mt-3">
                    {!s.occupied ? (
                      <Button type="button" size="sm" disabled={busySpot===s.id} onClick={async (e) => {
                        e.preventDefault();
                        try {
                          setBusySpot(s.id);
                          const res = await parkingService.reserveSpecific(p.id, s.id);
                          // optimistically update local state for the specific spot
                          setParks(prev => prev.map(pp => pp.id === p.id ? {
                            ...pp,
                            spots: pp.spots.map(sp => sp.id === s.id ? { ...sp, occupied: true, qrCode: res.qrBase64 } : sp)
                          } : pp));
                          // Add QR code to the map
                          setQrCodes(prev => ({ ...prev, [s.id]: { qr: res.qrBase64, parkName: p.name, spotNumber: s.number } }));
                        } catch (err: any) {
                          alert(err?.message || 'Failed to occupy spot');
                        } finally {
                          setBusySpot(null);
                        }
                      }}>Occupy</Button>
                    ) : (
                      <Button type="button" size="sm" variant="destructive" disabled={busySpot===s.id} onClick={async (e) => {
                        e.preventDefault();
                        try {
                          setBusySpot(s.id);
                          await parkingService.releaseSpot(p.id, s.id);
                          // optimistically update local state for the specific spot
                          setParks(prev => prev.map(pp => pp.id === p.id ? {
                            ...pp,
                            spots: pp.spots.map(sp => sp.id === s.id ? { ...sp, occupied: false, qrCode: undefined } : sp)
                          } : pp));
                          // Remove QR code from the map
                          setQrCodes(prev => {
                            const newCodes = { ...prev };
                            delete newCodes[s.id];
                            return newCodes;
                          });
                        } catch (err: any) {
                          alert(err?.message || 'Failed to release spot');
                        } finally {
                          setBusySpot(null);
                        }
                      }}>Release</Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}

        {Object.keys(qrCodes).length > 0 && (
          <div className="mt-6 p-4 border rounded">
            <h3 className="text-xl font-semibold mb-4">Your Reserved Parking QR Codes</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {Object.entries(qrCodes).map(([spotId, data]) => (
                <div key={spotId} className="p-4 border rounded-lg">
                  <h4 className="font-semibold mb-2">{data.parkName} - Spot #{data.spotNumber}</h4>
                  <img src={`data:image/png;base64,${data.qr}`} alt={`QR for ${data.parkName} spot ${data.spotNumber}`} className="w-full" />
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
