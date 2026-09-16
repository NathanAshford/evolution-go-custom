/**
 * TimelockTimer
 * Shows a live countdown until WhatsApp's reachout timelock ends for an instance.
 * While the timelock is active, the instance (a companion device) cannot start chats
 * with NEW contacts — sends return error 463. The component fetches the timelock from
 * GET /instance/limits/:id and counts down to `timeEnforcementEnds` locally.
 */

import { useEffect, useState } from "react";
import { Clock } from "lucide-react";
import {
  getInstanceLimits,
  type ReachoutTimelock,
} from "@/services/api/instances";

type Props = {
  instanceId: string;
  connected: boolean;
};

function formatRemaining(ms: number): string {
  if (ms <= 0) return "00:00:00";
  const totalSec = Math.floor(ms / 1000);
  const days = Math.floor(totalSec / 86400);
  const hours = Math.floor((totalSec % 86400) / 3600);
  const minutes = Math.floor((totalSec % 3600) / 60);
  const seconds = totalSec % 60;
  const pad = (n: number) => n.toString().padStart(2, "0");
  const hms = `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
  return days > 0 ? `${days}d ${hms}` : hms;
}

export default function TimelockTimer({ instanceId, connected }: Props) {
  const [timelock, setTimelock] = useState<ReachoutTimelock | null>(null);
  const [now, setNow] = useState(() => Date.now());

  // Fetch the timelock when the instance is connected.
  useEffect(() => {
    if (!connected) {
      setTimelock(null);
      return;
    }
    let cancelled = false;
    getInstanceLimits(instanceId)
      .then((limits) => {
        if (!cancelled) setTimelock(limits.reachoutTimelock);
      })
      .catch(() => {
        if (!cancelled) setTimelock(null);
      });
    return () => {
      cancelled = true;
    };
  }, [instanceId, connected]);

  const active = !!timelock?.isActive && timelock.timeEnforcementEnds > 0;

  // Tick every second while an active timelock is displayed.
  useEffect(() => {
    if (!active) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [active]);

  if (!active) return null;

  const endMs = timelock!.timeEnforcementEnds * 1000;
  const remaining = endMs - now;
  const ended = remaining <= 0;

  return (
    <div className="flex items-center justify-between rounded-md bg-amber-500/10 border border-amber-500/20 px-2 py-1.5 mt-2">
      <span className="flex items-center gap-1.5 text-amber-500">
        <Clock className="h-3.5 w-3.5" />
        <span className="font-medium">
          {ended ? "Timelock encerrando…" : "Timelock 463"}
        </span>
      </span>
      <span
        className="font-mono text-amber-400"
        title={`Envio a novos contatos liberado em ${new Date(
          endMs
        ).toLocaleString()} · ${timelock!.enforcementType}`}
      >
        {ended ? "00:00:00" : formatRemaining(remaining)}
      </span>
    </div>
  );
}
