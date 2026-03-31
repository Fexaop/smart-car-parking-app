import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { parkingService, Park, ParkingEvent, Spot } from "@/services/parking";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import Grainient from "@/components/Grainient";
import { AlertCircle, CheckCircle2, Clock, Lock, RefreshCw, Unlock, User } from "lucide-react";

type NoticeType = "success" | "error" | "info";

interface Notice {
  id: number;
  type: NoticeType;
  message: string;
}

interface OwnedSpot {
  key: string;
  parkId: string;
  parkName: string;
  spot: Spot;
}

export default function Dashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [parks, setParks] = useState<Park[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [notices, setNotices] = useState<Notice[]>([]);
  const [otpInputs, setOtpInputs] = useState<Record<string, string>>({});
  const [verifiedSpots, setVerifiedSpots] = useState<Record<string, boolean>>({});
  const [busySpots, setBusySpots] = useState<Record<string, boolean>>({});
  const [validatingSpots, setValidatingSpots] = useState<Record<string, boolean>>({});
  const [nowMs, setNowMs] = useState(Date.now());
  const [lastSyncedAt, setLastSyncedAt] = useState<Date | null>(null);
  const [wsConnected, setWsConnected] = useState(false);

  const getInitials = (name: string) => {
    return name
      .split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  };

  const pushNotice = (type: NoticeType, message: string) => {
    const id = Date.now() + Math.floor(Math.random() * 1000);
    setNotices((prev) => [...prev, { id, type, message }]);
    setTimeout(() => {
      setNotices((prev) => prev.filter((notice) => notice.id !== id));
    }, 3500);
  };

  const setSpotBusy = (spotId: string, value: boolean) => {
    setBusySpots((prev) => {
      const next = { ...prev };
      if (value) {
        next[spotId] = true;
      } else {
        delete next[spotId];
      }
      return next;
    });
  };

  const setSpotValidating = (spotId: string, value: boolean) => {
    setValidatingSpots((prev) => {
      const next = { ...prev };
      if (value) {
        next[spotId] = true;
      } else {
        delete next[spotId];
      }
      return next;
    });
  };

  const isOtpExpired = (expiry?: string): boolean => {
    if (!expiry) {
      return true;
    }
    return new Date(expiry).getTime() <= nowMs;
  };

  const getTimeRemaining = (expiry?: string): string => {
    if (!expiry) {
      return "No active OTP";
    }

    const diff = new Date(expiry).getTime() - nowMs;
    if (diff <= 0) {
      return "Expired";
    }

    const minutes = Math.floor(diff / 60000);
    const seconds = Math.floor((diff % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
  };

  const loadParks = async (silent = false) => {
    if (!silent) {
      setLoading(true);
    }

    try {
      const data = await parkingService.listParks();
      const sorted = data.sort((a, b) => {
        const aNum = parseInt(a.id.split("-")[1] || "0", 10);
        const bNum = parseInt(b.id.split("-")[1] || "0", 10);
        return aNum - bNum;
      });

      setParks(sorted);
      setLoadError(null);
      setLastSyncedAt(new Date());

      const activeOwnedKeys = new Set(
        sorted.flatMap((park) =>
          park.spots
            .filter((spot) => spot.ownedByCurrentUser)
            .map((spot) => `${park.id}:${spot.id}`)
        )
      );

      setOtpInputs((prev) => {
        const next: Record<string, string> = {};
        for (const [key, value] of Object.entries(prev)) {
          if (activeOwnedKeys.has(key)) {
            next[key] = value;
          }
        }
        return next;
      });

      setVerifiedSpots((prev) => {
        const next: Record<string, boolean> = {};
        for (const [key, value] of Object.entries(prev)) {
          if (activeOwnedKeys.has(key) && value) {
            next[key] = value;
          }
        }
        return next;
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to load parking";
      setLoadError(message);
      if (!silent) {
        pushNotice("error", message);
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    loadParks();

    const unsubscribe = parkingService.subscribeParkingEvents(
      (event: ParkingEvent) => {
        if (event.type === "spot_console_freed") {
          pushNotice("info", `Spot #${event.spot.number} was freed from live console.`);
        } else if (!event.actorIsCurrentUser && (event.type === "spot_reserved" || event.type === "spot_released")) {
          pushNotice("info", `Spot #${event.spot.number} changed from another user or device.`);
        }
        loadParks(true);
      },
      setWsConnected
    );

    const poll = setInterval(() => {
      loadParks(true);
    }, 15000);

    return () => {
      unsubscribe();
      clearInterval(poll);
    };
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      setNowMs(Date.now());
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  const ownedSpots = useMemo<OwnedSpot[]>(() => {
    return parks.flatMap((park) =>
      park.spots
        .filter((spot) => spot.ownedByCurrentUser)
        .map((spot) => ({
          key: `${park.id}:${spot.id}`,
          parkId: park.id,
          parkName: park.name,
          spot,
        }))
    );
  }, [parks]);

  const handleReserve = async (parkId: string, spotId: string) => {
    try {
      setSpotBusy(spotId, true);
      const res = await parkingService.reserveSpecific(parkId, spotId);
      setVerifiedSpots((prev) => {
        const next = { ...prev };
        delete next[`${parkId}:${spotId}`];
        return next;
      });
      setOtpInputs((prev) => ({ ...prev, [`${parkId}:${spotId}`]: res.otp || "" }));
      pushNotice("success", `Spot #${res.spotNumber} reserved successfully.`);
      await loadParks(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to reserve spot";
      pushNotice("error", message);
    } finally {
      setSpotBusy(spotId, false);
    }
  };

  const handleRelease = async (parkId: string, spotId: string) => {
    try {
      setSpotBusy(spotId, true);
      await parkingService.releaseSpot(parkId, spotId);
      setOtpInputs((prev) => {
        const next = { ...prev };
        delete next[`${parkId}:${spotId}`];
        return next;
      });
      setVerifiedSpots((prev) => {
        const next = { ...prev };
        delete next[`${parkId}:${spotId}`];
        return next;
      });
      pushNotice("success", `Spot #${spotId.split("-").pop()} released.`);
      await loadParks(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to release spot";
      pushNotice("error", message);
    } finally {
      setSpotBusy(spotId, false);
    }
  };

  const handleValidateOTP = async (parkId: string, spotId: string, key: string) => {
    const otp = (otpInputs[key] || "").trim();

    if (!/^\d{4}$/.test(otp)) {
      pushNotice("error", "OTP must be exactly 4 digits.");
      return;
    }

    try {
      setSpotValidating(spotId, true);
      await parkingService.validateOTP(parkId, spotId, otp);
      setVerifiedSpots((prev) => ({ ...prev, [key]: true }));
      setOtpInputs((prev) => ({ ...prev, [key]: "" }));
      pushNotice("success", `OTP verified for ${spotId}. Gate opened.`);
      await loadParks(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : "OTP validation failed";
      pushNotice("error", message);
    } finally {
      setSpotValidating(spotId, false);
    }
  };

  const handleToggleGate = async (parkId: string, spotId: string) => {
    try {
      setSpotBusy(spotId, true);
      await parkingService.openGate(parkId, spotId);
      pushNotice("info", `Gate command sent for ${spotId}.`);
      await loadParks(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to toggle gate";
      pushNotice("error", message);
    } finally {
      setSpotBusy(spotId, false);
    }
  };

  const handleClearOTPDisplay = async (parkId: string, spotId: string) => {
    try {
      setSpotBusy(spotId, true);
      await parkingService.clearOTPDisplay(parkId, spotId);
      pushNotice("info", `LCD cleared for ${spotId}.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to clear OTP display";
      pushNotice("error", message);
    } finally {
      setSpotBusy(spotId, false);
    }
  };

  return (
    <div className="relative isolate h-screen bg-background">
      <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 opacity-85">
          <div style={{ width: "2080px", height: "2080px", position: "relative" }}>
            <Grainient
              color1="#8647ff"
              color2="#5d00ff"
              color3="#060010"
              timeSpeed={0.25}
              colorBalance={0}
              warpStrength={2.75}
              warpFrequency={12}
              warpSpeed={2}
              warpAmplitude={29}
              blendAngle={-30}
              blendSoftness={0}
              rotationAmount={500}
              noiseScale={2}
              grainAmount={0.1}
              grainScale={2}
              grainAnimated
              contrast={1.5}
              gamma={1}
              saturation={1}
              centerX={0}
              centerY={0}
              zoom={0.9}
            />
          </div>
        </div>
      </div>

      <div className="relative z-10 p-4 sm:p-6">
        <div className="mx-auto max-w-7xl space-y-6">
        <Card className="border-border/50 bg-card/70 backdrop-blur">
          <CardContent className="pt-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h1 className="text-2xl font-bold tracking-tight">Parking Control Center</h1>
                <p className="text-sm text-muted-foreground">
                  Reserve, validate OTP, and control your owned gates across devices.
                </p>
              </div>
              <div className="flex items-center gap-3">
                <Button variant="outline" onClick={() => loadParks()} disabled={loading}>
                  <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
                  Refresh
                </Button>
                <button
                  onClick={() => navigate("/profile")}
                  className="flex items-center gap-2 rounded-xl border border-border/70 bg-background/70 px-2 py-1.5 transition hover:border-primary/50"
                >
                  <span className="hidden text-sm font-medium sm:block">{user?.name}</span>
                  <Avatar className="h-10 w-10 border border-primary/30">
                    <AvatarImage src={user?.picture} alt={user?.name} />
                    <AvatarFallback className="bg-primary text-primary-foreground">
                      {user?.name ? getInitials(user.name) : <User className="h-5 w-5" />}
                    </AvatarFallback>
                  </Avatar>
                </button>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              {lastSyncedAt && <p>Last synced at {lastSyncedAt.toLocaleTimeString()}</p>}
              <p className={wsConnected ? "text-emerald-500" : "text-amber-500"}>
                {wsConnected ? "Live updates connected" : "Live updates reconnecting"}
              </p>
            </div>
          </CardContent>
        </Card>

        {notices.length > 0 && (
          <div className="space-y-2">
            {notices.map((notice) => (
              <div
                key={notice.id}
                className={`flex items-start gap-2 rounded-lg border px-3 py-2 text-sm ${
                  notice.type === "success"
                    ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                    : notice.type === "error"
                      ? "border-red-500/40 bg-red-500/10 text-red-700 dark:text-red-300"
                      : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300"
                }`}
              >
                {notice.type === "success" ? (
                  <CheckCircle2 className="mt-0.5 h-4 w-4" />
                ) : (
                  <AlertCircle className="mt-0.5 h-4 w-4" />
                )}
                <span>{notice.message}</span>
              </div>
            ))}
          </div>
        )}

        {loadError && (
          <Card className="border-red-500/40 bg-red-500/10">
            <CardContent className="pt-6">
              <div className="flex items-center justify-between gap-3">
                <p className="text-sm text-red-700 dark:text-red-300">{loadError}</p>
                <Button variant="outline" size="sm" onClick={() => loadParks()}>
                  Retry
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">Available Parks</h2>
            {loading && <span className="text-sm text-muted-foreground">Loading...</span>}
          </div>
          {parks.map((park) => (
            <Card key={park.id} className="border-border/60 bg-card/80 backdrop-blur">
              <CardHeader>
                <CardTitle>{park.name}</CardTitle>
                <CardDescription>Choose a spot to reserve or manage your owned reservation.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {park.spots.map((spot) => {
                    const isBusy = Boolean(busySpots[spot.id]);
                    const owned = Boolean(spot.ownedByCurrentUser);
                    const cardTone = !spot.occupied
                      ? "border-emerald-400/45 bg-emerald-500/15"
                      : owned
                        ? "border-amber-400/55 bg-amber-500/18"
                        : "border-zinc-500/45 bg-zinc-700/30";
                    const statusText = !spot.occupied ? "Free" : owned ? "Yours" : "In use";

                    return (
                      <div key={spot.id} className={`rounded-xl border p-4 ${cardTone}`}>
                        <div className="mb-3 flex items-start justify-between">
                          <div>
                            <p className="text-lg font-semibold">Spot #{spot.number}</p>
                            <p className="text-sm text-muted-foreground">{spot.id}</p>
                          </div>
                          <span className="rounded-full bg-background/70 px-2 py-0.5 text-xs font-medium">
                            {statusText}
                          </span>
                        </div>

                        {!spot.occupied && (
                          <Button
                            className="w-full"
                            disabled={isBusy}
                            onClick={() => handleReserve(park.id, spot.id)}
                          >
                            {isBusy ? "Reserving..." : "Reserve"}
                          </Button>
                        )}

                        {spot.occupied && owned && (
                          <Button
                            className="w-full"
                            variant="destructive"
                            disabled={isBusy}
                            onClick={() => handleRelease(park.id, spot.id)}
                          >
                            {isBusy ? "Releasing..." : "Release"}
                          </Button>
                        )}

                        {spot.occupied && !owned && (
                          <Button className="w-full" variant="outline" disabled>
                            Unavailable
                          </Button>
                        )}
                      </div>
                    );
                  })}
                </div>
              </CardContent>
            </Card>
          ))}
        </section>

        <section className="space-y-4">
          <h2 className="text-xl font-semibold">My Spot Controls</h2>

          {ownedSpots.length === 0 && (
            <Card className="border-dashed border-border/70 bg-card/60">
              <CardContent className="pt-6 text-sm text-muted-foreground">
                You do not own any spots right now. Reserve a free spot to manage it.
              </CardContent>
            </Card>
          )}

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {ownedSpots.map(({ key, parkId, parkName, spot }) => {
              const isBusy = Boolean(busySpots[spot.id]);
              const isValidating = Boolean(validatingSpots[spot.id]);
              const requiresOtp = Boolean(spot.otp && !isOtpExpired(spot.otpExpiry));
              const otpVerified = Boolean(verifiedSpots[key]);
              const canControlGate = !requiresOtp || otpVerified;

              return (
                <Card key={key} className="border-border/60 bg-card/85 backdrop-blur">
                  <CardHeader>
                    <CardTitle className="flex items-center justify-between">
                      <span>{parkName} · Spot #{spot.number}</span>
                      {canControlGate ? (
                        <Unlock className="h-5 w-5 text-emerald-500" />
                      ) : (
                        <Lock className="h-5 w-5 text-amber-500" />
                      )}
                    </CardTitle>
                    <CardDescription className="flex items-center gap-2">
                      <Clock className="h-4 w-4" />
                      {requiresOtp ? `OTP expires in ${getTimeRemaining(spot.otpExpiry)}` : "No active OTP required"}
                    </CardDescription>
                  </CardHeader>

                  <CardContent className="space-y-3">
                    {requiresOtp && (
                      <div className="space-y-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
                        <p className="text-sm">Enter the 4-digit OTP shown on the parking LCD.</p>
                        <div className="flex gap-2">
                          <Input
                            type="text"
                            inputMode="numeric"
                            placeholder="4-digit OTP"
                            maxLength={4}
                            value={otpInputs[key] || ""}
                            onChange={(event) => {
                              const next = event.target.value.replace(/\D/g, "").slice(0, 4);
                              setOtpInputs((prev) => ({ ...prev, [key]: next }));
                            }}
                            className="text-center text-base tracking-[0.3em]"
                            disabled={isValidating}
                          />
                          <Button
                            onClick={() => handleValidateOTP(parkId, spot.id, key)}
                            disabled={isValidating || (otpInputs[key] || "").length !== 4}
                          >
                            {isValidating ? "Verifying..." : "Verify"}
                          </Button>
                        </div>
                        {otpVerified && (
                          <p className="text-xs text-emerald-600 dark:text-emerald-400">OTP verified for this session.</p>
                        )}
                      </div>
                    )}

                    {!canControlGate && (
                      <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-300">
                        Verify OTP to unlock gate controls for this active reservation.
                      </div>
                    )}

                    <div className="grid grid-cols-2 gap-2">
                      <Button
                        disabled={isBusy || !canControlGate}
                        onClick={() => handleToggleGate(parkId, spot.id)}
                      >
                        {isBusy ? "Sending..." : "Toggle Gate"}
                      </Button>
                      <Button
                        variant="outline"
                        disabled={isBusy || !canControlGate}
                        onClick={() => handleClearOTPDisplay(parkId, spot.id)}
                      >
                        Clear LCD
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </section>
        </div>
      </div>
    </div>
  );
}
